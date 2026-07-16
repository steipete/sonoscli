package cli

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestExecutable(t *testing.T, name, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("/bin/sh", "-c", `umask 077; cat > "$1"; chmod 700 "$1"`, "write-test-executable", path)
	cmd.Stdin = strings.NewReader(contents)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("write test executable: %v: %s", err, strings.TrimSpace(string(output)))
	}
	return path
}
