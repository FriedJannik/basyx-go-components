package common

import (
	"os"
	"strings"
	"testing"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	if _, err := file.WriteString(content); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}
	return file.Name()
}

func withUnsetEnv(t *testing.T, key string) {
	t.Helper()
	oldValue, hadValue := os.LookupEnv(key)
	if hadValue {
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
	}
	t.Cleanup(func() {
		if !hadValue {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, oldValue)
	})
}

func TestLoadConfigRejectsBooleanStrictVerification(t *testing.T) {
	withUnsetEnv(t, "SERVER_STRICTVERIFICATION")
	path := writeTempConfig(t, "server:\n  strictVerification: true\n")

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected invalid strictVerification mode error")
	}
	if !strings.Contains(err.Error(), "invalid server.strictVerification") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigAcceptsPermissiveStrictVerification(t *testing.T) {
	withUnsetEnv(t, "SERVER_STRICTVERIFICATION")
	path := writeTempConfig(t, "server:\n  strictVerification: permissive\n")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("unexpected config load error: %v", err)
	}
	if cfg.Server.StrictVerification != "permissive" {
		t.Fatalf("unexpected strictVerification mode: %q", cfg.Server.StrictVerification)
	}
}
