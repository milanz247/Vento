package config

import (
	"os"
	"testing"
)

func TestEnvReturnsSetValueOrFallback(t *testing.T) {
	t.Setenv("VENTO_TEST_ENV_STR", "hello")
	if got := Env("VENTO_TEST_ENV_STR", "default"); got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
	os.Unsetenv("VENTO_TEST_ENV_STR_MISSING")
	if got := Env("VENTO_TEST_ENV_STR_MISSING", "default"); got != "default" {
		t.Fatalf("expected fallback 'default', got %q", got)
	}
}

func TestEnvIntParsesOrFallsBack(t *testing.T) {
	t.Setenv("VENTO_TEST_ENV_INT", "42")
	if got := EnvInt("VENTO_TEST_ENV_INT", 1); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	t.Setenv("VENTO_TEST_ENV_INT_BAD", "not-a-number")
	if got := EnvInt("VENTO_TEST_ENV_INT_BAD", 7); got != 7 {
		t.Fatalf("expected fallback 7 for a non-numeric value, got %d", got)
	}
}

func TestEnvBoolParsesOrFallsBack(t *testing.T) {
	t.Setenv("VENTO_TEST_ENV_BOOL", "true")
	if got := EnvBool("VENTO_TEST_ENV_BOOL", false); !got {
		t.Fatal("expected true")
	}
	t.Setenv("VENTO_TEST_ENV_BOOL_BAD", "not-a-bool")
	if got := EnvBool("VENTO_TEST_ENV_BOOL_BAD", true); !got {
		t.Fatal("expected fallback true for a non-boolean value")
	}
}
