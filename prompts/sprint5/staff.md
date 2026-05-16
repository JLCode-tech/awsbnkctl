You are the **staff engineer** agent for Sprint 5 of awsbnkctl. Sprint 5 is primarily the book retarget; your scope is the small remaining staff work — chapter 22 image tag fix, IBM-residue tech-debt sweep, auto-regenerated reference chapter outputs, build green.

Project: `/Users/j.lucia/Code/github/awsbnkctl/`. Go module `github.com/JLCode-tech/awsbnkctl`.

**SPIKE DEFERRAL** carries.

**Read first:**
1. `agents/staff.md`
2. `prompts/sprint5/staff.md` (this) + `prompts/sprint5/README.md`
3. `docs/PLAN.md` § Sprint 5
4. Sprint 4 carry-overs: tech-writer Issue 1 (iperf3 image tag); Sprint 3 tech-writer Issue 2 (IBM residue)

## Off-limits

`docs/`, `book/src/{1-19,24-26,31-32}-*.md` (architect's prose retarget surface), `agents/`, `prompts/`, `.github/workflows/`, `cspell.json`, `scripts/` (validator scope).

Allowed: `book/src/{27,28,29,30}-*.md` (auto-regenerated reference chapters — but coordinate with architect).

## Your scope

| Surface | Action |
|---|---|
| `internal/k8s/iperf3.go` `Iperf3DefaultImage` | Sprint 4 tech-writer Issue 1: currently `networkstatic/iperf3:latest`; chapter 22 documents `awsbnkctl-tools-iperf3`. Pick one and align — recommend keeping `networkstatic/iperf3:latest` (it's a public image with iperf3 baked in) and let chapter 22 say so. Coordinate with architect via issue file if you change the constant; if you change the chapter, that's architect scope |
| `internal/cli/test.go` or wherever iperf3 image is referenced | Audit for consistency post-change |
| `internal/**` IBM-residue sweep | Sprint 3 tech-writer Issue 2: 302 hits across tests + comments. Sweep: delete obsolete `_test.go` files that referenced now-deleted IBM packages; retarget comments. Use grep to find hits: `grep -r 'IBMCloud\|IBMCLOUD\|ibmcloud\|roks\|ROKS\|COS\|cos\b' --include='*.go' internal/`. Don't break tests — if a test stays relevant, retarget; if it's obsolete, delete |
| `tools/refgen/cobra-md` output → `book/src/27-command-reference.md` (or wherever it lands) | Regenerate against current AWS-shaped CLI; commit the output |
| `tools/refgen/tfvars-md` output → `book/src/29-terraform-variable-reference.md` | Regenerate against current AWS variables; commit the output |
| Auto-gen workflow integration | If the auto-gen runs from Makefile or a script, run it; commit outputs |

## Tasks (priority order)

1. **iperf3 image tag decision** — align code + chapter
2. **IBM-residue sweep** — delete obsolete tests + retarget comments. Target: bring grep count under 50 (down from 302; remaining are tolerable in test-skip strings + migration notes)
3. **Reference chapter regeneration** — cobra-md + tfvars-md outputs
4. **Build green gate** — full suite (vet/fmt/build/test)
5. **File Sprint 5 staff issues**

## Verification

- `go vet / build / test / gofmt` clean
- `grep -r 'IBMCloud\|ibmcloud' --include='*.go' internal/ | wc -l` returns substantially less than 302
- `make book` or `mdbook build book/` succeeds against the regenerated reference chapters

## Final report

Under 200 words. Do NOT commit.
