// Package config loads workspace and global configuration, resolves the
// IBM Cloud API key (env / OS keychain / prompt), and renders Terraform
// variables files.
//
// File layout:
//
//	~/.awsbnkctl/config.yaml             — global preferences, current_workspace
//	~/.awsbnkctl/<workspace>/config.yaml — per-workspace inputs
//	~/.awsbnkctl/<workspace>/state/      — terraform.tfstate, kubeconfig, scratch/
//
// Override the base directory via $ROKSBNKCTL_HOME (used by tests; advanced
// users with non-home-dir state).
//
// Secrets policy: workspace config.yaml is rejected at load time if it
// contains plaintext credentials (api_key, password, token, etc.). The
// IBM Cloud API key lives in $IBMCLOUD_API_KEY or the OS keychain only.
package config
