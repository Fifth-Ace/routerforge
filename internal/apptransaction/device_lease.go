package apptransaction

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// DevicePackageLease is the same cooperative package lock used by Core. It
// intentionally does not claim to exclude older binaries or external opkg.
// The owner closes its descriptor, never explicitly unlocks it: a child may
// still own an inherited reference to the same open file description.
type DevicePackageLease struct {
	file   *os.File
	closed bool
}

func devicePackageLeaseRequired(goos, goarch, version string) bool {
	return goos == "linux" && goarch == "arm64" && strings.HasPrefix(version, "1~dev.r")
}

// AcquireDevicePackageLease uses the fixed device coordinator. Stable/Beta and
// other platforms keep their existing behavior, as does Core's admission gate.
func AcquireDevicePackageLease(version string) (*DevicePackageLease, error) {
	if !devicePackageLeaseRequired(runtime.GOOS, runtime.GOARCH, version) {
		return &DevicePackageLease{}, nil
	}
	return acquireDevicePackageLease()
}

// Attach is used only for the actual package manager, not a long-lived service.
// Descendant FD lifetime must be accepted on hardware before worker activation.
func (l *DevicePackageLease) Attach(cmd *exec.Cmd) error {
	if l == nil || l.closed || cmd == nil || cmd.Process != nil || len(cmd.ExtraFiles) != 0 {
		return errors.New("invalid package lease or command")
	}
	if l.file != nil {
		env, err := executionEnvironment(cmd.Environ(), "")
		if err != nil {
			return err
		}
		cmd.Env = env
		cmd.ExtraFiles = []*os.File{l.file}
	}
	return nil
}

func (l *DevicePackageLease) Close() {
	if l == nil || l.closed {
		return
	}
	l.closed = true
	if l.file != nil {
		_ = l.file.Close()
	}
}
