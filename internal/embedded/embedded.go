// Package embedded ships the agentic-mode scaffolding inside the binary.
//
// files/ is copied into a workspace by `awsbnkctl agent init`: AGENTS.md
// (the shared operator reference), personas/ (the role contracts),
// and decisions.md (the decision-log seed).
package embedded

import "embed"

// FS holds everything under files/.
//
//go:embed files
var FS embed.FS
