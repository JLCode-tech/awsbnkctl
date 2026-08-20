#!/usr/bin/env python3
"""Render the three governed paths as an SVG, for slides.

  python3 scripts/build-diagram.py            -> images/three-paths.svg
  python3 scripts/build-diagram.py --out X.svg

No dependencies. Convert to PNG with:
  rsvg-convert -w 2400 images/three-paths.svg -o images/three-paths.png
"""

import argparse
import pathlib
import xml.sax.saxutils as xml

W, H = 1180, 760
INK = "#1c2530"
MUTED = "#6b7785"
LINE = "#9aa5b1"
F5 = "#e4002b"
AWS = "#ff9900"
OK = "#127c3f"
DENY = "#b4232c"
BG = "#ffffff"
PANEL = "#f6f8fa"

out = []


def esc(t):
    return xml.escape(str(t))


def text(x, y, s, size=13, fill=INK, weight="normal", anchor="start", mono=False):
    fam = ("ui-monospace,SFMono-Regular,Menlo,monospace" if mono
           else "-apple-system,Segoe UI,Inter,Helvetica,Arial,sans-serif")
    out.append(
        f'<text x="{x}" y="{y}" font-family="{fam}" font-size="{size}" fill="{fill}" '
        f'font-weight="{weight}" text-anchor="{anchor}">{esc(s)}</text>'
    )


def box(x, y, w, h, label, sub=None, stroke=LINE, fill="#fff", dash=None, lw=1.4):
    d = f' stroke-dasharray="{dash}"' if dash else ""
    out.append(
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="7" fill="{fill}" '
        f'stroke="{stroke}" stroke-width="{lw}"{d}/>'
    )
    cy = y + h / 2 + (0 if sub is None else -7)
    text(x + w / 2, cy + 4, label, 12.5, INK, "600", "middle")
    if sub:
        text(x + w / 2, cy + 21, sub, 10.5, MUTED, "normal", "middle", mono=True)


def frame(x, y, w, h, stroke=LINE, fill="#fff", dash=None, lw=1.4):
    d = f' stroke-dasharray="{dash}"' if dash else ""
    out.append(
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="7" fill="{fill}" '
        f'stroke="{stroke}" stroke-width="{lw}"{d}/>'
    )


def arrow(x1, y1, x2, y2, label=None, colour=LINE, dash=None, above=True):
    d = f' stroke-dasharray="{dash}"' if dash else ""
    out.append(
        f'<line x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}" stroke="{colour}" '
        f'stroke-width="1.6" marker-end="url(#a)"{d}/>'
    )
    if label:
        mx, my = (x1 + x2) / 2, (y1 + y2) / 2
        text(mx, my - 7 if above else my + 15, label, 10, MUTED, anchor="middle", mono=True)


def panel(x, y, w, h, n, title, status, colour):
    out.append(
        f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="10" fill="{PANEL}" '
        f'stroke="#e3e8ee" stroke-width="1"/>'
    )
    out.append(f'<circle cx="{x+22}" cy="{y+24}" r="11" fill="{colour}"/>')
    text(x + 22, y + 28, n, 12, "#fff", "700", "middle")
    text(x + 42, y + 29, title, 14, INK, "700")
    text(x + w - 14, y + 29, status, 11, MUTED, "600", "end")


def build():
    out.append(
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W} {H}" width="{W}" height="{H}">'
    )
    out.append(
        f'<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" '
        f'markerHeight="7" orient="auto-start-reverse">'
        f'<path d="M 0 0 L 10 5 L 0 10 z" fill="{LINE}"/></marker></defs>'
    )
    out.append(f'<rect width="{W}" height="{H}" fill="{BG}"/>')

    text(40, 44, "Governing the agentic action path", 23, INK, "700")
    text(40, 68, "Amazon Bedrock AgentCore  ×  F5 BIG-IP Next for Kubernetes", 13, MUTED)
    text(W - 40, 68, "the tool hop is the one hop you control", 11.5, F5, "600", "end")

    # ── Path 1 ───────────────────────────────────────────────────────────────
    py = 96
    panel(40, py, W - 80, 178, "1", "Trusted agent path", "runs today", OK)
    box(70, py + 52, 168, 62, "AgentCore Runtime", "ENI 10.0.11.15", AWS)
    box(302, py + 34, 150, 52, "Amazon Bedrock", "stopReason: tool_use", AWS)
    arrow(238, py + 62, 302, py + 50, "1. reason", AWS)
    arrow(302, py + 74, 240, py + 80, "4. narrate", AWS, above=False)
    frame(520, py + 40, 210, 104, F5, "#fff", None, 2)
    text(625, py + 64, "F5 BNK  (TMM)", 12.5, INK, "600", "middle")
    text(625, py + 84, "VIP 10.0.10.100", 10.5, MUTED, anchor="middle", mono=True)
    text(625, py + 102, "authn · authz · rate · log", 10, MUTED, anchor="middle", mono=True)
    text(625, py + 126, "ONLY CHECKPOINT", 9.5, F5, "700", "middle")
    arrow(238, py + 100, 520, py + 100, "2. tools/call", LINE, above=False)
    box(792, py + 62, 150, 58, "MCP tool pod", "forecast()", LINE)
    arrow(730, py + 92, 792, py + 92, "3. $", OK)
    text(1000, py + 84, "AgentCore is not in", 10.5, MUTED)
    text(1000, py + 99, "the tool hop, by design.", 10.5, MUTED)

    # ── Path 2 ───────────────────────────────────────────────────────────────
    py = 296
    panel(40, py, W - 80, 168, "2", "Double-checked path", "not built", MUTED)
    box(70, py + 54, 168, 58, "AgentCore Runtime", None, AWS)
    box(292, py + 44, 186, 78, "AgentCore Gateway", "Cedar · guardrails", AWS, dash="5 4")
    text(385, py + 108, "identity-aware per-tool authz", 9.5, MUTED, anchor="middle", mono=True)
    arrow(238, py + 82, 292, py + 82, "MCP", LINE)
    box(532, py + 52, 176, 62, "VPC Lattice", "resource gateway ENIs", AWS, dash="5 4")
    arrow(478, py + 82, 532, py + 82)
    box(762, py + 44, 186, 78, "F5 BNK  (TMM)", "network + workload policy", F5, dash="5 4", lw=2)
    arrow(708, py + 82, 762, py + 82)
    box(998, py + 54, 132, 58, "MCP tool pod", None, LINE, dash="5 4")
    arrow(948, py + 82, 998, py + 82)
    text(70, py + 140, "Two independent policy engines, different questions. Blocked on: no Gateway deployed, "
                        "and a publicly trusted cert for the target.", 10.5, MUTED)

    # ── Path 3 ───────────────────────────────────────────────────────────────
    py = 486
    panel(40, py, W - 80, 200, "3", "Stranger path", "runs today", OK)
    box(70, py + 62, 176, 62, "Unmanaged caller", "another cloud · a script", MUTED)
    frame(330, py + 34, 250, 130, F5, "#fff", None, 2)
    text(455, py + 54, "F5 BNK  (TMM)", 12.5, INK, "600", "middle")
    for i, (line, col) in enumerate([
        ("401  no / wrong credential", DENY),
        ("403  privileged tool", DENY),
        ("429  rate limit exceeded", DENY),
        ("---  non-VPC source rejected", DENY),
        ("200  forecast allowed", OK),
    ]):
        text(348, py + 78 + i * 17, line, 10.5, col, "600", mono=True)
    arrow(246, py + 99, 330, py + 99, "POST", LINE)
    box(646, py + 70, 150, 58, "MCP tool pod", None, LINE)
    arrow(580, py + 99, 646, py + 99)
    text(830, py + 74, "No AgentCore component is in", 11, INK, "600")
    text(830, py + 91, "this path — no JWT check, no", 10.5, MUTED)
    text(830, py + 106, "Cedar, no guardrail. This is the", 10.5, MUTED)
    text(830, py + 121, "case a managed front door", 10.5, MUTED)
    text(830, py + 136, "cannot cover.", 10.5, MUTED)

    text(40, H - 16, "Verified live on bnk-agentcore-demo, ap-southeast-2. "
                     "Dashed = not yet built.", 10, MUTED)
    out.append("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default=None)
    args = ap.parse_args()
    dest = pathlib.Path(args.out) if args.out else \
        pathlib.Path(__file__).resolve().parent.parent / "images" / "three-paths.svg"
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(build(), encoding="utf-8")
    print(f"wrote {dest} ({dest.stat().st_size:,} bytes)")
