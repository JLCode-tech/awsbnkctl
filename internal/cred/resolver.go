// Package cred is the single source of truth for credential resolution
// across awsbnkctl execution backends.
//
// # Background
//
// Pre-Sprint-3, the IBM Cloud API key was resolved by
// internal/config.ResolveAPIKey() — a free function called from
// internal/cli/cluster.go and internal/cli/lifecycle.go. As Sprint 3 adds
// the local + docker execution backends and Sprint 4 adds k8s + ssh, every
// backend needs a canonical "give me the API key" entry point that can be
// stubbed in tests, share-by-instance across a single command invocation,
// and (later) carry per-backend cred policy hints.
//
// PRD 04 §"Open questions" §"Centralized cred resolver" picks the
// single-resolver design. This package implements it. The legacy
// config.ResolveAPIKey() free function is kept as a thin shim so
// existing call sites that haven't been refactored yet continue to work.
//
// Resolution chain (for IBM Cloud):
//
//  1. environment — IBMCLOUD_API_KEY / IC_API_KEY / TF_VAR_ibmcloud_api_key /
//     TF_VAR_IBMCLOUD_API_KEY / TF_VAR_IC_API_KEY (first non-empty wins)
//  2. OS keychain — service="awsbnkctl", user="<workspace>/ibmcloud_api_key"
//  3. workspace config.yaml — api_key_b64 (base64-decoded)
//  4. interactive prompt (TTY) — skipped if NonInteractive=true or stdin
//     isn't a terminal
//
// PRD 04 cross-backend principle #1: never log credentials. The resolver
// returns the value to the caller and lets caller decide how to use it
// (env var, bind mount file content, etc.); error messages from the
// resolver itself never embed candidate values.
package cred

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	"github.com/JLCode-tech/awsbnkctl/internal/config"
)

// keychainService is the OS-keychain "service" namespace — must match the
// constant in internal/config/secrets.go so a key written by the legacy
// resolver round-trips through this one.
const keychainService = "awsbnkctl"

// apiKeyEnvVars lists the env var names (in order) the resolver consults.
// Mirrors the legacy list in internal/config/secrets.go so behaviour is
// byte-identical pre/post extraction.
var apiKeyEnvVars = []string{
	"IBMCLOUD_API_KEY",
	"IC_API_KEY",
	"TF_VAR_ibmcloud_api_key",
	"TF_VAR_IBMCLOUD_API_KEY",
	"TF_VAR_IC_API_KEY",
}

// Resolver implements the chain described in the package comment.
//
// Workspace selects which keychain entry / config.yaml file to consult;
// must be a valid workspace name (config.ValidateName-compatible) for the
// keychain + config-file paths to resolve. An empty Workspace skips the
// keychain + config-file steps and relies solely on env / prompt.
//
// NonInteractive=true makes step 4 (prompt) a hard error rather than
// blocking on stdin. CI / non-TTY automation must set this; the local CLI
// leaves it false so users get the friendly TTY prompt on first run.
//
// PromptOut + PromptIn are optional injection seams used by tests; nil
// means "use os.Stderr / os.Stdin". Production callers pass nil.
type Resolver struct {
	Workspace      string
	NonInteractive bool

	// Source overrides the chain when non-empty. Same semantics as the
	// legacy config.ResolveAPIKey's source param: "env" | "keychain" |
	// "config" | "prompt". Most callers leave this empty.
	Source string
}

// IBMCloudAPIKey returns the resolved IBM Cloud API key for r.Workspace.
//
// The context is not yet load-bearing (no I/O is cancellable today) but
// is part of the signature so future backends — k8s Secret read, IBM IAM
// trusted-profile assume — can hang request timeouts off it without an
// API change.
func (r *Resolver) IBMCloudAPIKey(ctx context.Context) (string, error) {
	if r == nil {
		return "", errors.New("nil cred.Resolver")
	}
	switch r.Source {
	case "":
		// Default chain — fall through to the per-step calls below.
	case "env":
		if k, ok := apiKeyFromEnv(); ok {
			return k, nil
		}
		return "", errEnvMiss()
	case "keychain":
		k, err := apiKeyFromKeychain(r.Workspace)
		if err != nil {
			return "", err
		}
		if k == "" {
			return "", fmt.Errorf("no IBM Cloud API key for workspace %q in OS keychain", r.Workspace)
		}
		return k, nil
	case "config":
		k, err := apiKeyFromConfig(r.Workspace)
		if err != nil {
			return "", err
		}
		if k == "" {
			return "", fmt.Errorf("no api_key_b64 set in workspace %q config.yaml", r.Workspace)
		}
		return k, nil
	case "prompt":
		if r.NonInteractive {
			return "", errors.New("api_key_source=prompt but resolver is non-interactive")
		}
		return apiKeyFromPrompt(r.Workspace)
	default:
		return "", fmt.Errorf("unknown api_key_source %q (want env|keychain|config|prompt)", r.Source)
	}

	// Default chain.
	if k, ok := apiKeyFromEnv(); ok {
		return k, nil
	}
	if r.Workspace != "" {
		if k, err := apiKeyFromKeychain(r.Workspace); err == nil && k != "" {
			return k, nil
		}
		if k, err := apiKeyFromConfig(r.Workspace); err == nil && k != "" {
			return k, nil
		}
	}
	if r.NonInteractive {
		return "", errNonInteractiveMiss(r.Workspace)
	}
	return apiKeyFromPrompt(r.Workspace)
}

// errEnvMiss is the canonical "no API key in env" error. Stored here so
// tests can match against a stable shape if they want.
func errEnvMiss() error {
	return errors.New("no IBM Cloud API key in environment (looked for IBMCLOUD_API_KEY, IC_API_KEY, TF_VAR_ibmcloud_api_key, TF_VAR_IBMCLOUD_API_KEY, TF_VAR_IC_API_KEY)")
}

// errNonInteractiveMiss is the canonical "every chain source empty + can't
// prompt" error. Mentions every place a user could put the key so the
// message is actionable.
func errNonInteractiveMiss(ws string) error {
	if ws == "" {
		return errors.New("no IBM Cloud API key available (set IBMCLOUD_API_KEY env var)")
	}
	return fmt.Errorf("no IBM Cloud API key available for workspace %q (set IBMCLOUD_API_KEY env var, store in OS keychain via `awsbnkctl init`, or add api_key_b64 to config.yaml)", ws)
}

func apiKeyFromEnv() (string, bool) {
	for _, v := range apiKeyEnvVars {
		if k := os.Getenv(v); k != "" {
			return k, true
		}
	}
	return "", false
}

// apiKeyFromKeychain returns the keychain entry for the workspace, or ""
// if none. Missing workspace name short-circuits to "" without an error
// — the caller's chain falls through to the next step.
func apiKeyFromKeychain(workspace string) (string, error) {
	if workspace == "" {
		return "", nil
	}
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

// apiKeyFromConfig formerly read IBMCloud.APIKeyB64 from the workspace
// config.yaml. Sprint 3 (PRD 04 retarget) dropped the IBMCloud schema
// block; AWS credentials resolve via the SDK chain in internal/aws,
// never via the workspace config. Retained as a no-op so the existing
// chain in IBMCloudAPIKey keeps compiling — it always returns empty so
// callers fall through to the (now also unused) prompt step.
func apiKeyFromConfig(workspace string) (string, error) {
	if workspace == "" {
		return "", nil
	}
	if _, err := config.LoadWorkspace(workspace); err != nil {
		if errors.Is(err, config.ErrWorkspaceNotFound) {
			return "", nil
		}
		return "", err
	}
	return "", nil
}

// apiKeyFromPrompt reads the key from the TTY without echo, then offers to
// save it to the OS keychain. Errors out cleanly if stdin isn't a TTY so
// non-interactive callers don't hang.
func apiKeyFromPrompt(workspace string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		if workspace == "" {
			return "", errors.New("no IBM Cloud API key available and stdin is not a TTY (cannot prompt; set IBMCLOUD_API_KEY)")
		}
		return "", fmt.Errorf("no IBM Cloud API key for workspace %q and stdin is not a TTY (cannot prompt; set IBMCLOUD_API_KEY or run `awsbnkctl init`)", workspace)
	}
	wsLabel := workspace
	if wsLabel == "" {
		wsLabel = "default"
	}
	fmt.Fprintf(os.Stderr, "Enter IBM Cloud API key for workspace %q: ", wsLabel)
	keyBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading API key: %w", err)
	}
	key := strings.TrimSpace(string(keyBytes))
	if key == "" {
		return "", errors.New("empty API key")
	}

	if workspace != "" {
		fmt.Fprintf(os.Stderr, "Save the key for future runs? [Y/n]: ")
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer == "" || strings.HasPrefix(answer, "y") {
			dest, serr := config.SaveAPIKeyForWorkspace(workspace, key)
			if serr != nil {
				fmt.Fprintf(os.Stderr, "  warning: could not persist key now (%v); the calling command may retry after the workspace is created\n", serr)
			} else {
				fmt.Fprintf(os.Stderr, "  saved to %s\n", dest)
			}
		}
	}
	return key, nil
}
