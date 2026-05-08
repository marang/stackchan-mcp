package secretstore

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

var ErrNotFound = errors.New("secret not found")

func Lookup(attrs ...string) (string, error) {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return "", errors.New("secret-tool is required for Secret Service storage")
	}
	cmd := exec.Command("secret-tool", append([]string{"lookup"}, attrs...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", ErrNotFound
	}
	return string(bytes.TrimSpace(out)), nil
}

func Store(label string, secret string, attrs ...string) error {
	if _, err := exec.LookPath("secret-tool"); err != nil {
		return errors.New("secret-tool is required for Secret Service storage")
	}
	args := append([]string{"store", "--label", label}, attrs...)
	cmd := exec.Command("secret-tool", args...)
	cmd.Stdin = strings.NewReader(secret)
	return cmd.Run()
}
