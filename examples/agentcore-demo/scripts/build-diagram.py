#!/usr/bin/env python3
"""Render the demo's diagrams as SVGs, for slides.

  python3 scripts/build-diagram.py     -> images/three-paths.svg
                                       -> images/estate-token-governance.svg
                                       -> images/network-and-paths.svg

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
    out.clear()
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
    box(70, py + 52, 168, 62, "AgentCore Runtime", "ENIs .11.15 / .12.191", AWS)
    text(70, py + 130, "multi-homed — key limits on", 9.5, MUTED)
    text(70, py + 144, "caller identity, not client IP", 9.5, MUTED)
    box(302, py + 34, 150, 52, "Amazon Bedrock", "stopReason: tool_use", AWS)
    arrow(238, py + 62, 302, py + 50, "1. reason", AWS)
    arrow(302, py + 74, 240, py + 80, "4. narrate", AWS, above=False)
    frame(520, py + 40, 210, 104, F5, "#fff", None, 2)
    text(625, py + 64, "F5 BNK  (TMM)", 12.5, INK, "600", "middle")
    text(625, py + 84, "VIP 10.0.10.100", 10.5, MUTED, anchor="middle", mono=True)
    text(625, py + 102, "authz · rate · firewall · log", 10, MUTED, anchor="middle", mono=True)
    text(625, py + 126, "ONLY CHECKPOINT", 9.5, F5, "700", "middle")
    arrow(238, py + 100, 520, py + 100, "2. tools/call", LINE, above=False)
    box(792, py + 62, 150, 58, "MCP tool pod", "forecast()", LINE)
    arrow(730, py + 92, 792, py + 92, "3. fact", OK)
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
    text(455, py + 52, "F5 BNK  (TMM)", 12.5, INK, "600", "middle")
    text(455, py + 68, "refused before the pod", 9.5, MUTED, anchor="middle", mono=True)
    for i, (line, col) in enumerate([
        ("403  privileged tool", DENY),
        ("429  rate limit exceeded", DENY),
        ("---  non-VPC source rejected", DENY),
        ("200  forecast → forwarded", OK),
    ]):
        text(348, py + 90 + i * 17, line, 10.5, col, "600", mono=True)
    arrow(246, py + 99, 330, py + 99, "POST", LINE)
    box(646, py + 70, 150, 58, "MCP tool pod", None, LINE)
    arrow(580, py + 99, 646, py + 99)
    text(646, py + 148, "401  no / wrong credential", 10.5, DENY, "600", mono=True)
    text(646, py + 163, "issued by the tool, not BNK", 9.5, MUTED)
    text(830, py + 74, "No AgentCore component is in", 11, INK, "600")
    text(830, py + 91, "this path — no JWT check, no", 10.5, MUTED)
    text(830, py + 106, "Cedar, no guardrail. This is the", 10.5, MUTED)
    text(830, py + 121, "case a managed front door", 10.5, MUTED)
    text(830, py + 136, "cannot cover.", 10.5, MUTED)

    text(40, H - 16, "Verified live on bnk-agentcore-demo, ap-southeast-2. "
                     "Dashed = not yet built.", 10, MUTED)
    out.append("</svg>")
    return "\n".join(out)




def build_estate():
    """BNK enforces in-path; Forge accounts for everything AWS logs."""
    out.clear()
    W2, H2 = 1220, 720
    out.append(
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W2} {H2}" width="{W2}" height="{H2}">'
    )
    out.append(
        f'<defs><marker id="a" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" '
        f'markerHeight="7" orient="auto-start-reverse">'
        f'<path d="M 0 0 L 10 5 L 0 10 z" fill="{LINE}"/></marker></defs>'
    )
    out.append(f'<rect width="{W2}" height="{H2}" fill="{BG}"/>')

    text(40, 44, "Token governance across the AWS AI estate", 23, INK, "700")
    text(40, 68, "BNK enforces where it is in the path. Forge accounts for everything AWS logs. "
                 "Together they cover the estate.", 13, MUTED)

    # (label, AWS TPM covers it, BNK can be in path, AWS writes CloudWatch logs)
    rows = [
        ("Agent via AgentCore Gateway", True, True, True),
        ("Agents on EKS / ECS / EC2", False, True, True),
        ("Self-hosted vLLM / NIM on GPU nodes", False, True, False),
        ("Third-party LLM APIs (OpenAI, ...)", False, True, False),
        ("SageMaker endpoints", False, True, True),
        ("Apps calling Bedrock over the AWS backbone", False, False, True),
    ]
    text(40, 112, "WHERE THE TOKENS ARE BURNED", 10.5, MUTED, "700")
    y0 = 130
    for i, (label, tpm, inpath, cwlogs) in enumerate(rows):
        y = y0 + i * 44
        stroke = LINE if inpath else "#d8dee5"
        fill = "#fff" if inpath else "#f4f6f8"
        frame(40, y, 322, 34, stroke, fill, None if inpath else "4 3")
        text(52, y + 22, label, 11, INK if inpath else MUTED, "600")
        if tpm:
            frame(370, y + 5, 84, 24, AWS, "#fff8ee", None, 1.2)
            text(412, y + 21, "RPM·TPM·CPS", 8, "#8a5200", "700", "middle")
            text(412, y + 40, "AWS covers this row", 8.5, MUTED, anchor="middle")
        if inpath:
            arrow(462, y + 17, 543, 208, None, F5)
        elif cwlogs:
            arrow(462, y + 17, 543, 372, None, "#c9d2db", "3 3")

    # lane A: BNK, in-path enforcement
    frame(545, 122, 232, 172, F5, "#fff", None, 2)
    text(661, 148, "F5 BNK", 14, INK, "700", "middle")
    text(661, 165, "in path \u2014 CAN STOP IT", 9.5, F5, "700", "middle")
    for i, line in enumerate([
        "per-user / per-model limits",
        "429 when budget is spent",
        "counts in dSSM",
    ]):
        text(560, 190 + i * 20, "\u2022 " + line, 10, INK, mono=True)
    text(661, 262, "parses OpenAI-shaped usage;", 9, DENY, "600", "middle")
    text(661, 276, "Bedrock-native meters ZERO today", 9, DENY, "600", "middle")

    # lane B: CloudWatch, out-of-path accounting
    frame(545, 316, 232, 112, "#c9d2db", "#f8fafb", "4 3", 1.4)
    text(661, 342, "CloudWatch logs", 13, INK, "700", "middle")
    text(661, 359, "out of path \u2014 SEES ONLY", 9.5, MUTED, "700", "middle")
    text(560, 382, "\u2022 Bedrock invocation logs", 10, INK, mono=True)
    text(560, 400, "\u2022 real inputTokens / outputTokens", 10, INK, mono=True)

    # Forge joins both
    frame(838, 178, 210, 214, LINE, "#fff", None, 1.8)
    text(943, 206, "BNK Forge", 14, INK, "700", "middle")
    text(943, 224, "the join point", 10, MUTED, anchor="middle")
    for i, line in enumerate([
        "BNK: who was allowed,",
        "  throttled, refused",
        "AWS: what it cost",
        "  in real tokens",
        "one view, whole estate",
    ]):
        text(852, 250 + i * 19, line, 10, INK, mono=True)
    arrow(779, 208, 836, 245, None, F5)
    arrow(779, 372, 836, 330, None, "#9aa5b1")

    box(838, 424, 210, 50, "Chargeback", "per team \u00b7 per model", LINE)
    arrow(943, 392, 943, 424)

    # the point
    frame(40, 500, 1140, 84, "#e3e8ee", PANEL, None, 1)
    text(60, 528, "\u201cWhat is our AI spend, by team, and who is about to blow the budget?\u201d",
         13, INK, "700")
    text(60, 552, "The Gateway limiter answers it for one row. BNK + Forge answers it for every row \u2014 "
                  "enforcing on the ones BNK fronts,", 11, MUTED)
    text(60, 570, "accounting for the rest from AWS's own logs.", 11, MUTED)

    text(40, 618, "HONEST LIMITS", 10.5, DENY, "700")
    for i, line in enumerate([
        "Enforcement needs BNK in the path. Rows it does not front are visible but not stoppable.",
        "BNK token counting parses OpenAI-shaped usage; Bedrock-native responses meter as zero today (F5 feature request).",
        "The CloudWatch lane is accounting after the fact \u2014 it reports spend, it cannot prevent it.",
        "AgentCore Gateway limits are real and cover requests, tokens AND connections \u2014 but only on its own path,",
        "   scoped to /v1/chat|messages|responses, excluding pass-through targets, and they fail open by design.",
    ]):
        text(40, 638 + i * 18, ("\u2022 " if not line.startswith("  ") else "") + line, 10.5, MUTED)

    out.append("</svg>")
    return "\n".join(out)


def build_network():
    """VPC / subnet / component layout, with the three paths drawn on it.

    Every address here was read off the live cluster on 2026-08-20. If the
    cluster is rebuilt, re-check them before reusing the diagram — subnet CIDRs
    are stable (awsbnkctl assigns them) but ENI and pod addresses are not.
    """
    out.clear()
    W4, H4 = 1460, 1030
    TINT = "#fff5f6"          # BNK-owned subnets
    out.append(
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {W4} {H4}" width="{W4}" height="{H4}">'
    )
    marks = "".join(
        f'<marker id="m{k}" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" '
        f'markerHeight="7" orient="auto-start-reverse">'
        f'<path d="M 0 0 L 10 5 L 0 10 z" fill="{v}"/></marker>'
        for k, v in (("l", LINE), ("f", F5), ("o", OK), ("i", INK))
    )
    out.append(f"<defs>{marks}</defs>")
    out.append(f'<rect width="{W4}" height="{H4}" fill="{BG}"/>')

    mk = {LINE: "ml", F5: "mf", OK: "mo", INK: "mi"}

    def ar(pts, colour=LINE, dash=None, label=None, lx=None, ly=None, lanchor="middle"):
        """Arrow along a polyline of (x, y) points."""
        d = f' stroke-dasharray="{dash}"' if dash else ""
        p = " ".join(f"{x},{y}" for x, y in pts)
        out.append(
            f'<polyline points="{p}" fill="none" stroke="{colour}" stroke-width="2" '
            f'marker-end="url(#{mk[colour]})"{d}/>'
        )
        if label:
            mx = lx if lx is not None else (pts[0][0] + pts[-1][0]) / 2
            my = ly if ly is not None else (pts[0][1] + pts[-1][1]) / 2
            text(mx, my, label, 10, colour, "700", lanchor, mono=True)

    def rows(x, y, items, gap=18, size=10.5):
        for i, s in enumerate(items):
            text(x, y + i * gap, s, size, MUTED, mono=True)

    text(40, 44, "Where BNK sits in the VPC", 23, INK, "700")
    text(40, 68, "bnk-agentcore-demo · vpc-06dbebcc9fe3e1bad · 10.0.0.0/16 · ap-southeast-2 · "
                 "addresses read live 2026-08-20", 12.5, MUTED)

    # ── AWS-managed, outside the VPC ─────────────────────────────────────────
    text(40, 112, "AWS-MANAGED · OUTSIDE YOUR VPC", 10, MUTED, "700")
    box(40, 124, 210, 54, "Amazon Bedrock", "Converse · tool_use", AWS)
    box(40, 196, 210, 58, "AgentCore Runtime", "microVM", AWS)
    box(40, 272, 210, 56, "AgentCore Gateway", "not deployed", MUTED, dash="5 4")
    box(40, 346, 210, 56, "VPC Lattice res-gw", "ENIs in your subnets", MUTED, dash="5 4")
    ar([(145, 328), (145, 344)], LINE, dash="4 3")
    text(40, 424, "Path 2 is blocked here: AgentCore", 10, DENY, "600")
    text(40, 438, "will not trust our in-cluster CA.", 10, DENY, "600")

    # ── the VPC ──────────────────────────────────────────────────────────────
    frame(280, 96, 1140, 736, LINE, "#fbfcfd", None, 1.6)
    text(296, 120, "VPC  10.0.0.0/16", 13, INK, "700")
    text(1404, 120, "IGW igw-00ca152dbf662b6a9", 10, MUTED, anchor="end", mono=True)

    # Band A — private subnets, where AWS puts its ENIs
    for fx, name, cidr, az, items in (
        (296, "subnet-private-1", "10.0.11.0/24", "2a",
         ["AgentCore runtime ENI  10.0.11.15   agentic_ai",
          "EKS control plane ENI  10.0.11.20",
          "AgentCore Lambda ENI   10.0.11.228"]),
        (859, "subnet-private-2", "10.0.12.0/24", "2b",
         ["AgentCore runtime ENI  10.0.12.191  agentic_ai",
          "EKS control plane ENI  10.0.12.193",
          "AgentCore Lambda ENI   10.0.12.248"]),
    ):
        frame(fx, 140, 545, 128, LINE, PANEL)
        text(fx + 12, 160, f"{name} · {cidr} · {az}", 11, INK, "700")
        rows(fx + 12, 182, items)
    text(871, 254, "the same logical agent as private-1", 10.5, DENY, "600", mono=True)

    # Band B — bnk-ext: the VIP lives here
    frame(296, 288, 1108, 122, F5, TINT, None, 1.6)
    text(308, 310, "subnet-bnk-ext · 10.0.10.0/24 · 2a", 11, INK, "700")
    text(1392, 310, "BNK's external side", 10, F5, "700", "end")
    box(320, 324, 230, 66, "VIP  10.0.10.100", ":80  ·  :443 TLS", F5, "#fff", lw=2.4)
    box(596, 324, 230, 66, "TMM  ens8", "10.0.10.209", F5)
    box(872, 324, 230, 66, "jumphost", "10.0.10.29", LINE)

    # Band C — bnk-int: TMM's second NIC
    frame(296, 438, 1108, 88, F5, TINT, None, 1.6)
    text(308, 460, "subnet-bnk-int · 10.0.20.0/24 · 2a", 11, INK, "700")
    text(1392, 460, "BNK's internal side", 10, F5, "700", "end")
    box(596, 470, 230, 44, "TMM  ens7", "10.0.20.171", F5)
    text(848, 486, "same f5-tmm pod, second NIC — DPDK over vfio.", 10.5, MUTED)
    text(848, 502, "This is the host-device pattern: TMM owns both interfaces.", 10.5, MUTED)

    # Band D — public subnets: the nodes and the pods
    frame(296, 554, 745, 184, LINE, PANEL)
    text(308, 574, "subnet-public-1 · 10.0.1.0/24 · 2a", 11, INK, "700")
    text(1029, 574, "nodes + pods", 10, MUTED, anchor="end")
    box(312, 588, 228, 58, "node 10.0.1.11", "f5-tmm", F5)
    box(556, 588, 228, 58, "node 10.0.1.32", "—", LINE)
    box(800, 588, 226, 58, "node 10.0.1.181", "mcp-financial-tool", LINE)
    box(800, 660, 226, 44, "mcp pod 10.0.1.62", None, OK)
    text(312, 676, "pod IPs come from the subnet —", 10, MUTED)
    text(312, 691, "VPC CNI prefix delegation.", 10, MUTED)
    text(312, 726, "NAT 10.0.1.176 · EICE 10.0.1.33 · jumphost eth0 10.0.1.187", 10, MUTED, mono=True)
    frame(1059, 554, 345, 184, LINE, PANEL, "4 3")
    text(1071, 574, "subnet-public-2 · 10.0.2.0/24 · 2b", 11, MUTED, "700")
    text(1071, 598, "no workloads — capacity for a", 10.5, MUTED)
    text(1071, 614, "second AZ, unused by the demo.", 10.5, MUTED)

    # in-VPC services strip
    frame(296, 756, 1108, 58, LINE, "#fff")
    text(308, 780, "Route 53 private zone", 11, INK, "700")
    text(308, 798, "bnk-demo.internal  →  10.0.10.100", 10.5, MUTED, mono=True)
    text(700, 780, "NAT gateway", 11, INK, "700")
    text(700, 798, "nat-041ed9c3206186bac  (egress only)", 10.5, MUTED, mono=True)
    text(1080, 780, "Security groups", 11, INK, "700")
    text(1080, 798, "SG-to-SG rule lets the runtime ENIs reach TMM", 10.5, MUTED, mono=True)

    # ── the paths ────────────────────────────────────────────────────────────
    # Path 1: runtime ENIs (both AZs) -> VIP. Both start below the subnet frame
    # so the line never crosses the addresses it is coming from.
    # Entry points sit to the right of the subnet label so the lines do not
    # cross it.
    ar([(505, 270), (505, 322)], OK, label="path 1", lx=513, ly=266, lanchor="start")
    ar([(1200, 270), (1200, 284), (538, 284), (538, 322)], OK, dash="6 4")
    # Path 3: jumphost -> VIP. Routed through the gap below band B rather than
    # straight across, which would run through the TMM box.
    ar([(987, 392), (987, 424), (435, 424), (435, 392)], INK,
       label="path 3", lx=711, ly=420)
    # Path 2: Lattice ENIs -> VIP (not built)
    ar([(252, 374), (300, 374), (300, 352), (318, 352)], F5, dash="6 4",
       label="path 2", lx=262, ly=338, lanchor="start")
    # BNK internals: VIP -> ens8 -> ens7 -> node -> pod
    ar([(552, 357), (594, 357)], F5)
    ar([(711, 392), (711, 468)], F5)
    ar([(711, 516), (711, 540), (913, 540), (913, 584)], F5, label="to pod", lx=840, ly=534)
    ar([(913, 648), (913, 658)], LINE)

    # ── legend ───────────────────────────────────────────────────────────────
    ly0 = 866
    text(40, ly0 - 16, "THE THREE PATHS", 10.5, MUTED, "700")
    for i, (colour, dash, name, desc) in enumerate((
        (OK, None, "1  trusted agent",
         "AgentCore Runtime ENI → VIP → pod.  Runs today."),
        (F5, "6 4", "2  double-checked",
         "AgentCore Gateway → Lattice ENIs → VIP → pod.  Not built."),
        (INK, None, "3  stranger",
         "any caller that can route to the VIP → pod.  Runs today."),
    )):
        y = ly0 + i * 26
        d = f' stroke-dasharray="{dash}"' if dash else ""
        out.append(f'<line x1="40" y1="{y}" x2="86" y2="{y}" stroke="{colour}" '
                   f'stroke-width="2.4" marker-end="url(#{mk[colour]})"{d}/>')
        text(98, y + 4, name, 11.5, INK, "700")
        text(240, y + 4, desc, 11, MUTED)

    text(40, 972, "Everything inside the VPC frame is ours to control. Paths 1 and 3 terminate "
                  "on the same VIP and the same route — one policy surface, whatever door the "
                  "traffic came", 11, MUTED)
    text(40, 988, "through. Nothing reaches the pod without passing TMM. The runtime is "
                  "multi-homed across both AZs, so one logical agent presents two source IPs — "
                  "which is why the", 11, MUTED)
    text(40, 1004, "rate limiter keys on caller identity, not client IP.", 11, MUTED)
    out.append("</svg>")
    return "\n".join(out)


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    ap.add_argument("--outdir", default=None)
    args = ap.parse_args()
    outdir = pathlib.Path(args.outdir) if args.outdir else \
        pathlib.Path(__file__).resolve().parent.parent / "images"
    outdir.mkdir(parents=True, exist_ok=True)
    for name, fn in (("three-paths.svg", build),
                     ("estate-token-governance.svg", build_estate),
                     ("network-and-paths.svg", build_network)):
        dest = outdir / name
        dest.write_text(fn(), encoding="utf-8")
        print(f"wrote {dest} ({dest.stat().st_size:,} bytes)")
