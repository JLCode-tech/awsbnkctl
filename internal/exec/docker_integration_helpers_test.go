//go:build integration

package exec

import (
	"context"
	"os/exec"
	"time"
)

// dockerAvailable reports whether the local docker daemon is reachable.
// Integration tests use this to skip cleanly when the test host doesn't
// have docker (CI matrix entries without a daemon, dev laptops with the
// daemon stopped). 2-second timeout — `docker info` is normally <100ms
// when the daemon is alive.
func dockerAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", "info").Run() == nil // #nosec G204 -- "docker" + "info" are hard-coded literals, no taint
}
