# Sprint 0

**Theme:** identity rewrite + IBM strip + AWS stub

_Drafted from `docs/PLAN.md` Sprint 0 section._

Sprint 0 turns the freshly-forked `roksbnkctl` tree into `awsbnkctl`: rewrites the repo identity (module path, binary name, top-level docs), strips IBM-specific surface, and stubs the AWS-specific surface so subsequent sprints have a clean base to layer onto. End-of-sprint gate: `make build && make test && go vet ./...` all green; `awsbnkctl --help` runs and prints the expected command list; no `ibmcloud` / `roks` / `cos` string references remain in the build path (allowed in `book/`, `CHANGELOG.md` historical sections, and `MIGRATING.md` cross-references).

Carry-overs from the fork point (`jgruberf5/roksbnkctl@v1.2.1`): none — Sprint 0 starts from a clean fork.

Four-agent dispatch (parallel):

1. **architect** — verifies and finalises the prose surface already drafted in the working tree (`README.md`, `CHANGELOG.md`, `MIGRATING.md`, `docs/PLAN.md`, `docs/prd/00-OVERVIEW.md`, `docs/prd/07-EKS-CLUSTER-SRIOV.md`); touches up `agents/` role-definition wording where it references roksbnkctl specifics.
2. **staff** — does the implementation work: Go module path rewrite, binary rename, IBM-package deletion, AWS-package stubs, Terraform module rewiring, build-and-test green gate.
3. **validator** — updates CI workflows, cspell dictionary, e2e drivers, tools-image matrix; gates that the test suite stays green after the strip.
4. **tech-writer** — read-only at sprint close: dogfood the renamed binary, walk the README → PLAN → PRDs cross-link graph, file findings.

The integrator (human or another agent) folds the four agents' output into a single identity-rewrite commit on `main`.
