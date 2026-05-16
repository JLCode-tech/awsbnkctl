package config

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keychainService is the OS-keychain "service" namespace awsbnkctl uses.
//
// Inherited from the pre-Sprint-3 IBM-cred flow. AWS retarget (PRD 04):
// production code resolves AWS credentials via the SDK chain in
// internal/aws and never writes to or reads from the OS keychain. The
// only function exported from this file today is
// DeleteAPIKeyFromKeychain, called from `awsbnkctl workspaces delete`
// to remove any residual entry left over from a v0.x install.
const keychainService = "awsbnkctl"

// DeleteAPIKeyFromKeychain removes a workspace's legacy keychain entry,
// if one is present. Best-effort: missing entries are not an error.
//
// Kept post-Sprint-5 IBM-residue sweep as a one-time migration helper —
// a user upgrading from a v0.x install (when the keychain stored the
// IBM Cloud API key) will have residue from `awsbnkctl init`; deleting
// the workspace cleans it up. New installs never write to the keychain.
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
