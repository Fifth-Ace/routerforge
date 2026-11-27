//go:build linux

package apptransaction

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const MutationLeaseFD = 3
const mutationLockName = "package-mutation.lock"

// AcquireMutationLease is a candidate shared exclusion protocol. ALL package
// mutation paths must adopt this exact lock directory before production use.
// This does not exclude external opkg processes that do not cooperate.
func AcquireMutationLease(dir string) (*os.File, error) {
	s, err := Open(dir)
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open(filepath.Join(s.dir, mutationLockName), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), mutationLockName)
	st, err := f.Stat()
	if err != nil || !privateRegular(st) {
		f.Close()
		return nil, errors.New("unsafe mutation lock")
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, ErrBusy
		}
		return nil, err
	}
	return f, nil
}

// AdoptMutationLease validates the inherited open file description against the
// private canonical lock path. Close, never LOCK_UN: another process can hold
// the same open file description during the parent-to-worker handoff.
func AdoptMutationLease(dir string, fd uintptr) (*os.File, error) {
	if fd != MutationLeaseFD {
		return nil, errors.New("invalid inherited lease descriptor")
	}
	s, err := Open(dir)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(fd, mutationLockName)
	if f == nil {
		return nil, errors.New("missing inherited lease")
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
		}
	}()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) {
		return nil, errors.New("unsafe inherited lease")
	}
	path, err := os.Lstat(filepath.Join(s.dir, mutationLockName))
	if err != nil || !privateRegular(path) || !os.SameFile(st, path) {
		return nil, errors.New("inherited lease identity mismatch")
	}
	if err = syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, err
	}
	syscall.CloseOnExec(int(fd))
	ok = true
	return f, nil
}

type workerRequest struct {
	Plan       Plan   `json:"plan"`
	PlanSHA256 string `json:"plan_sha256"`
}

func durableExclusive(s *Store, name string, b []byte) error {
	path := filepath.Join(s.dir, name)
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
func ReadWorkerRequest(dir string) (Plan, error) {
	s, err := Open(dir)
	if err != nil {
		return Plan{}, err
	}
	fd, err := syscall.Open(filepath.Join(s.dir, "request.json"), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return Plan{}, err
	}
	f := os.NewFile(uintptr(fd), "request.json")
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !privateRegular(st) || st.Size() > maxJournal {
		return Plan{}, errors.New("unsafe worker request")
	}
	var r workerRequest
	d := json.NewDecoder(io.LimitReader(f, maxJournal+1))
	d.DisallowUnknownFields()
	if err = d.Decode(&r); err != nil {
		return Plan{}, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Plan{}, errors.New("trailing request data")
	}
	h, err := r.Plan.Hash()
	if err != nil || h != r.PlanSHA256 {
		return Plan{}, errors.New("worker request integrity mismatch")
	}
	return r.Plan, nil
}

// StartDetached starts a trusted, fixed executable selected by the integrating
// service, never by an API client. Its worker entry point must adopt fd 3,
// read and verify request.json, and execute the plan. A failed spawn leaves a
// durable request and requires explicit recovery. No shell or context-bound
// command; setsid isolates the worker from the launching session. This is not
// a guarantee against supervisor/cgroup-wide termination or a reboot.
func StartDetached(dir, lockDir, executable string, args []string, p Plan) (int, error) {
	p.Packages = append([]Package(nil), p.Packages...)
	h, err := p.Hash()
	if err != nil {
		return 0, err
	}
	s, err := Open(dir)
	if err != nil {
		return 0, err
	}
	if dir == lockDir {
		return 0, errors.New("worker and shared lock directories must differ")
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return 0, errors.New("worker executable must be absolute")
	}
	lease, err := AcquireMutationLease(lockDir)
	if err != nil {
		return 0, err
	}
	defer lease.Close()
	local, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer local.Close()
	// Never launch over an existing request or checkpoint, even if terminal.
	if _, err = s.Load(); err == nil {
		return 0, ErrRecovery
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	b, err := json.Marshal(workerRequest{p, h})
	if err != nil {
		return 0, err
	}
	if err = durableExclusive(s, "request.json", b); err != nil {
		return 0, err
	}
	fd, err := syscall.Open(filepath.Join(s.dir, "worker.log"), syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return 0, err
	}
	log := os.NewFile(uintptr(fd), "worker.log")
	defer log.Close()
	// Release store lock before child executes; global lease and exclusive
	// request protect startup. No other direct Execute caller is permitted.
	if err = local.Close(); err != nil {
		return 0, err
	}
	cmd := exec.Command(executable, args...)
	cmd.Dir = s.dir
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.ExtraFiles = []*os.File{lease}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	go func() { _ = cmd.Wait() }()
	return pid, nil
}
