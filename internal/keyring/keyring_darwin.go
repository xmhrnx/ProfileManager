//go:build darwin

package keyring

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// The Keychain is driven through /usr/bin/security, the tool most
// command-line programs (Claude Code, gh through go-keyring) use
// themselves, so entries keep the access rules those programs expect.
const securityTool = "/usr/bin/security"

type system struct{}

// errItemNotFound is security's exit status for a missing entry.
const errItemNotFound = 44

func (system) Get(it Item) ([]byte, error) {
	args := []string{"find-generic-password", "-s", it.Service}
	if it.Account != "" {
		args = append(args, "-a", it.Account)
	}
	args = append(args, "-w")
	var out, stderr bytes.Buffer
	cmd := exec.Command(securityTool, args...)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == errItemNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("keychain: %s", strings.TrimSpace(stderr.String()))
	}
	return bytes.TrimSuffix(out.Bytes(), []byte("\n")), nil
}

// Set writes through security's interactive mode, reading the command
// from standard input, so the secret never appears in a process list.
func (system) Set(it Item, value []byte) error {
	line := fmt.Sprintf("add-generic-password -U -s %s -a %s", quote(it.Service), quote(it.Account))
	if it.Label != "" {
		line += " -l " + quote(it.Label)
	}
	line += " -X " + hex.EncodeToString(value) + "\n"
	var stderr bytes.Buffer
	cmd := exec.Command(securityTool, "-i")
	cmd.Stdin = strings.NewReader(line)
	cmd.Stderr = &stderr
	// Interactive mode exits 0 even when the command fails; failures
	// are reported on standard error.
	if err := cmd.Run(); err != nil || strings.TrimSpace(stderr.String()) != "" {
		return fmt.Errorf("keychain: could not save %q: %s", it.Service, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func (system) Delete(it Item) error {
	args := []string{"delete-generic-password", "-s", it.Service}
	if it.Account != "" {
		args = append(args, "-a", it.Account)
	}
	var stderr bytes.Buffer
	cmd := exec.Command(securityTool, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == errItemNotFound {
			return nil
		}
		return fmt.Errorf("keychain: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

// quote writes s as a double-quoted argument for security -i.
func quote(s string) string { return strconv.Quote(s) }
