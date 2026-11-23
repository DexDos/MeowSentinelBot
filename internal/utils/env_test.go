package utils

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// 1. Variable not set
	_ = os.Unsetenv("TEST_VAR_NONEXISTENT")
	val, err := GetEnv("TEST_VAR_NONEXISTENT")
	if err == nil || val != "" {
		t.Errorf("expected error for unset variable, got val=%q, err=%v", val, err)
	}

	// 2. Variable set via system / container environment (no .env file required)
	testVal := "secret_token_12345"
	_ = os.Setenv("TEST_BOT_TOKEN", testVal)
	defer os.Unsetenv("TEST_BOT_TOKEN")

	val, err = GetEnv("TEST_BOT_TOKEN")
	if err != nil {
		t.Fatalf("unexpected error for set variable: %v", err)
	}
	if val != testVal {
		t.Errorf("expected %q, got %q", testVal, val)
	}
}
