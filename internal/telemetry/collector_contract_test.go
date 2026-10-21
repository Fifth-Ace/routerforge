package telemetry

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// The normal Go CI suite also executes the standalone collector's stdlib tests.
func TestCollectorPythonContract(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("collector is tested in Linux CI")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable on this test host")
	}
	dir := filepath.Join("..", "..", "services", "telemetry-collector")
	if _, err := os.Stat(filepath.Join(dir, "collector.py")); err != nil {
		t.Skip("collector not present in isolated client unit-test workspace")
	}
	cmd := exec.Command(py, "-m", "unittest", "discover", "-s", "tests", "-v")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("collector tests failed: %v\n%s", err, out)
	}
}
