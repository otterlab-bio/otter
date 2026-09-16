// Package verify checks installed binaries and their version output.
package verify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// VersionPattern matches the accepted release version formats.
var VersionPattern = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?|[0-9]{4}\.[0-9]{2}\.[0-9]{2}\.[0-9]+|daily-[0-9]{8})$`)

// Binary checks that a binary exists, is executable, and reports a valid
// version. It returns the version output.
func Binary(ctx context.Context, installDir, name string, dryRun bool) (string, error) {
	path := filepath.Join(installDir, name)
	if dryRun {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("%s not found at %s", name, path)
	}
	if info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable at %s", name, path)
	}
	command := exec.CommandContext(ctx, path, "--version")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s --version failed: %w", name, err)
	}
	firstLine := strings.TrimSpace(strings.SplitN(string(output), "\n", 2)[0])
	return firstLine, nil
}
