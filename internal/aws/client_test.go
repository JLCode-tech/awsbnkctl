package aws

import (
	"context"
	"os"
	"testing"
)

func TestHasEnvCredentials_Env(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_PROFILE", "")
	if !HasEnvCredentials() {
		t.Fatal("expected true when AWS_ACCESS_KEY_ID is set")
	}
}

func TestHasEnvCredentials_Profile(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_PROFILE", "myprof")
	if !HasEnvCredentials() {
		t.Fatal("expected true when AWS_PROFILE is set")
	}
}

func TestHasEnvCredentials_Neither(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_PROFILE", "")
	if HasEnvCredentials() {
		t.Fatal("expected false when neither is set")
	}
}

func TestNewClients_RegionRequired(t *testing.T) {
	// Strip every region-bearing env var so the chain has nothing to
	// fall back on. Setting AWS_REGION="" overrides the user shell.
	for _, v := range []string{"AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE"} {
		t.Setenv(v, "")
	}
	// HOME must not point at a profile that ships a region — stub.
	t.Setenv("AWS_CONFIG_FILE", "/nonexistent")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/nonexistent")

	if _, err := NewClients(context.Background(), Options{}); err == nil {
		// On a CI / dev host AWS_REGION might leak in from elsewhere;
		// only fail when we know region resolution should have come up
		// empty.
		if os.Getenv("AWS_REGION") == "" && os.Getenv("AWS_DEFAULT_REGION") == "" {
			t.Fatal("expected error when no region is configured")
		}
	}
}
