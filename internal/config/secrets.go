package config

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

// API key resolution sources. Sprint 3 (PRD 04 retarget) drops the
// IBM-named env/keychain/config plumbing from production paths;
// AWS resolves via the SDK chain (env / shared config / profile /
// instance role / SSO) and never via a workspace-config field.
// These constants persist as integration-test fixtures referenced
// from the inherited cred resolver tests that are themselves
// scheduled for retirement in Sprint 4 alongside the cred-shim.
const (
	APIKeySourceEnv      = "env"
	APIKeySourceKeychain = "keychain"
	APIKeySourceConfig   = "config" // base64-encoded in workspace config.yaml — obfuscation only
	APIKeySourcePrompt   = "prompt"

	// keychainService is the OS-keychain "service" namespace awsbnkctl uses.
	// Per-workspace entries persist as "<workspace>/ibmcloud_api_key" for
	// inherited resolver tests; new code does not consult the keychain.
	keychainService = "awsbnkctl"
)

// apiKeyEnvVars persists as a fixture for the inherited cred resolver
// test surface. Production code consults AWS credentials via the SDK
// chain in internal/aws, never this list.
var apiKeyEnvVars = []string{
	"IBMCLOUD_API_KEY",
	"IC_API_KEY",
	"TF_VAR_ibmcloud_api_key",
	"TF_VAR_IBMCLOUD_API_KEY",
	"TF_VAR_IC_API_KEY",
}

// ResolveAPIKey is the legacy API key resolver. As of Sprint 4, all
// production callers have migrated to cred.Resolver — this function
// remains only as a transitional shim used by package-local tests in
// context_test.go (which can't import cred without breaking the
// dependency graph: cred imports config, not the other way around).
//
// New code MUST use cred.Resolver. This shim will be deleted once the
// package-local tests are reorganised (e.g., moved to cred or rewritten
// to exercise the lower-level apiKeyFromConfig/Env/Keychain helpers
// directly).
//
// source overrides the resolution chain when non-empty:
//
//	""         — env → keychain → config (base64) → prompt → error
//	"env"      — env only
//	"keychain" — keychain only
//	"config"   — base64-decoded api_key_b64 in workspace config.yaml only
//	"prompt"   — interactive prompt only (errors if stdin is not a TTY)
//
// Deprecated: Use cred.Resolver in new code.
func ResolveAPIKey(workspace, source string) (string, error) {
	switch source {
	case "":
		if k, ok := apiKeyFromEnv(); ok {
			return k, nil
		}
		if k, err := apiKeyFromKeychain(workspace); err == nil && k != "" {
			return k, nil
		}
		if k, err := apiKeyFromConfig(workspace); err == nil && k != "" {
			return k, nil
		}
		return apiKeyFromPrompt(workspace)
	case APIKeySourceEnv:
		if k, ok := apiKeyFromEnv(); ok {
			return k, nil
		}
		return "", errors.New("no IBM Cloud API key in environment (looked for IBMCLOUD_API_KEY, IC_API_KEY, TF_VAR_ibmcloud_api_key, TF_VAR_IBMCLOUD_API_KEY, TF_VAR_IC_API_KEY)")
	case APIKeySourceKeychain:
		k, err := apiKeyFromKeychain(workspace)
		if err != nil {
			return "", err
		}
		if k == "" {
			return "", fmt.Errorf("no API key for workspace %q in OS keychain", workspace)
		}
		return k, nil
	case APIKeySourceConfig:
		k, err := apiKeyFromConfig(workspace)
		if err != nil {
			return "", err
		}
		if k == "" {
			return "", fmt.Errorf("no api_key_b64 set in workspace %q config.yaml", workspace)
		}
		return k, nil
	case APIKeySourcePrompt:
		return apiKeyFromPrompt(workspace)
	default:
		return "", fmt.Errorf("unknown api_key_source %q (want env|keychain|config|prompt)", source)
	}
}

func apiKeyFromEnv() (string, bool) {
	for _, v := range apiKeyEnvVars {
		if k := os.Getenv(v); k != "" {
			return k, true
		}
	}
	return "", false
}

// apiKeyFromConfig formerly read the legacy IBMCloud.APIKeyB64 field
// from the workspace config.yaml. Sprint 3 dropped that schema; AWS
// credentials resolve via the SDK chain and never appear in the
// workspace file. Retained as a no-op shim so the inherited resolver
// chain keeps compiling; always returns empty so callers fall through
// to the next source.
func apiKeyFromConfig(workspace string) (string, error) {
	if _, err := LoadWorkspace(workspace); err != nil {
		if errors.Is(err, ErrWorkspaceNotFound) {
			return "", nil
		}
		return "", err
	}
	return "", nil
}

// EncodeAPIKeyForConfig base64-encodes a plaintext API key for storage
// in IBMCloudCfg.APIKeyB64. Convenience for callers (e.g. `awsbnkctl init
// --save-api-key` in v1.x); users can also encode by hand:
//
//	echo -n "$IBMCLOUD_API_KEY" | base64
func EncodeAPIKeyForConfig(plaintext string) string {
	return base64.StdEncoding.EncodeToString([]byte(plaintext))
}

func apiKeyFromKeychain(workspace string) (string, error) {
	user := workspace + "/ibmcloud_api_key"
	k, err := keyring.Get(keychainService, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("reading OS keychain: %w", err)
	}
	return k, nil
}

// apiKeyFromPrompt reads the key from the TTY without echo, then offers to
// save it to the OS keychain.
func apiKeyFromPrompt(workspace string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("no IBM Cloud API key available and stdin is not a TTY (cannot prompt; set IBMCLOUD_API_KEY or run `awsbnkctl init`)")
	}
	fmt.Fprintf(os.Stderr, "Enter IBM Cloud API key for workspace %q: ", workspace)
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading API key: %w", err)
	}
	key := strings.TrimSpace(string(keyBytes))
	if key == "" {
		return "", errors.New("empty API key")
	}

	fmt.Fprintf(os.Stderr, "Save the key for future runs? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" || strings.HasPrefix(answer, "y") {
		dest, err := SaveAPIKeyForWorkspace(workspace, key)
		if err != nil {
			// Both keychain and config save failed. Most common reason:
			// workspace doesn't exist yet (init flow). The caller will
			// re-attempt persistence after the workspace is saved.
			fmt.Fprintf(os.Stderr, "  warning: could not persist key now (%v); the calling command may retry after the workspace is created\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "  ✓ saved to %s\n", dest)
		}
	}
	return key, nil
}

// SaveAPIKeyToKeychain stores the API key under the awsbnkctl service for the
// given workspace. Used by `awsbnkctl init` once the user has entered the key.
func SaveAPIKeyToKeychain(workspace, key string) error {
	if err := ValidateName(workspace); err != nil {
		return err
	}
	user := workspace + "/ibmcloud_api_key"
	return keyring.Set(keychainService, user, key)
}

// APIKeyInKeychain reports whether the workspace already has a key
// stored in the OS keychain. Used by callers that want to decide
// whether to also persist via config.yaml b64.
func APIKeyInKeychain(workspace string) bool {
	k, err := apiKeyFromKeychain(workspace)
	return err == nil && k != ""
}

// SaveAPIKeyForWorkspace persists the key to the most reliable
// destination available. Order:
//
//  1. OS keychain (recommended — process-isolated, system-managed).
//  2. config.yaml api_key_b64 (fallback for environments without a
//     working keychain — typically WSL2 without libsecret).
//
// Returns the destination it wrote to, or an error if both failed
// (e.g. keychain unavailable AND workspace doesn't exist yet — caller
// should retry after creating the workspace).
//
// Idempotent: calling repeatedly with the same key is safe.
func SaveAPIKeyForWorkspace(workspace, key string) (string, error) {
	if kerr := SaveAPIKeyToKeychain(workspace, key); kerr == nil {
		return "OS keychain", nil
	} else if cerr := saveAPIKeyToConfig(workspace, key); cerr == nil {
		return "config.yaml (base64)", nil
	} else {
		return "", fmt.Errorf("keychain failed (%v) and config save failed: %w", kerr, cerr)
	}
}

// saveAPIKeyToConfig formerly persisted api_key_b64 into the workspace
// config.yaml under the IBMCloud block. Sprint 3 dropped the block;
// AWS credentials resolve via the SDK chain and never via the
// workspace config. Retained as a no-op error stub so the inherited
// keychain-fallback shim in SaveAPIKeyForWorkspace keeps compiling.
func saveAPIKeyToConfig(workspace, _ string) error {
	return fmt.Errorf("config-file API-key persistence retired in Sprint 3 (PRD 04 retarget): AWS credentials resolve via the SDK chain for workspace %q", workspace)
}

// DeleteAPIKeyFromKeychain removes the workspace's keychain entry. Used
// by `awsbnkctl workspaces delete` to leave no residue. Missing entry is
// not an error.
func DeleteAPIKeyFromKeychain(workspace string) error {
	if err := ValidateName(workspace); err != nil {
		return err
	}
	user := workspace + "/ibmcloud_api_key"
	err := keyring.Delete(keychainService, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
