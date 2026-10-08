"""Generate assets/demo.svg: an animated terminal walkthrough of ghostwriter.

    python3 assets/make_demo_svg.py assets/demo.svg

The scores shown are real. They come from
`ghostwriter score draft.md -g email --source brief.md` against a profile built
from internal/voice/testdata/writer, where draft.md is the text of scene 2 (pass
1) or scene 3 (pass 2), with blank lines between paragraphs, and brief.md is:

    Reply to Marta: yes to Thursday's demo. Slides by Wednesday. Payment API sandbox still down.

Re-run them and update the numbers here if scoring changes.
"""
import sys
from html import escape

T = 27.0          # loop length, seconds
W, H = 880, 520
X0, LH = 36, 21   # left margin, line height
FONT = "ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace"

C = {
    "fg": "#e6edf3", "dim": "#8b949e", "green": "#3fb950", "prompt": "#7ee787",
    "red": "#ff7b72", "blue": "#79c0ff", "purple": "#d2a8ff", "yellow": "#e3b341",
    "draft": "#c9d1d9",
}

els = []      # (svg, css)
n = 0


def pct(t):
    return f"{max(0.0, min(100.0, t / T * 100)):.3f}%"


def line(y, segs, start, end, type_dur=0.0, final=False, size=14, weight=None):
    """A line of text visible from start to end; optionally typed out.

    segs: list of (text, color, flag_at) where flag_at, if set, is the time
    the segment turns red (an AI tell being flagged).
    final: shown when animation is off (reduced motion / static render).
    """
    global n
    n += 1
    cls = f"e{n}"
    spans = []
    css = []
    for i, (text, color, flag_at) in enumerate(segs):
        scls = f"{cls}s{i}"
        spans.append(f'<tspan class="{scls}" fill="{C[color]}">{escape(text)}</tspan>')
        if flag_at is not None:
            css.append(
                f"@keyframes {scls}k{{0%,{pct(flag_at)}{{fill:{C[color]}}}"
                f"{pct(flag_at + 0.2)},100%{{fill:{C['red']}}}}}"
                f".{scls}{{animation:{scls}k {T}s linear infinite}}"
            )
    chars = sum(len(s[0]) for s in segs)
    fade_out = end + 0.25
    if type_dur:
        k = (
            f"@keyframes {cls}k{{"
            f"0%,{pct(start - 0.01)}{{opacity:0;clip-path:inset(0 100% 0 0)}}"
            f"{pct(start)}{{opacity:1;clip-path:inset(0 100% 0 0);animation-timing-function:steps({chars},end)}}"
            f"{pct(start + type_dur)},{pct(end)}{{opacity:1;clip-path:inset(0 0 0 0)}}"
            f"{pct(fade_out)},100%{{opacity:0;clip-path:inset(0 0 0 0)}}}}"
        )
    else:
        k = (
            f"@keyframes {cls}k{{0%,{pct(start)}{{opacity:0}}"
            f"{pct(start + 0.15)},{pct(end)}{{opacity:1}}{pct(fade_out)},100%{{opacity:0}}}}"
        )
    css.append(k)
    css.append(f".{cls}{{opacity:{1 if final else 0};animation:{cls}k {T}s linear infinite}}")
    attrs = f' font-size="{size}"' + (f' font-weight="{weight}"' if weight else "")
    els.append((f'<text class="{cls}" x="{X0}" y="{y}"{attrs} xml:space="preserve">{"".join(spans)}</text>', "".join(css)))


def box(y, h, start, end, final=False, at=None):
    """A panel behind the draft. at: index in els to insert at, so the panel
    is painted underneath lines that were added before its height was known."""
    global n
    n += 1
    cls = f"e{n}"
    css = (
        f"@keyframes {cls}k{{0%,{pct(start)}{{opacity:0}}{pct(start + 0.15)},{pct(end)}{{opacity:1}}"
        f"{pct(end + 0.25)},100%{{opacity:0}}}}"
        f".{cls}{{opacity:{1 if final else 0};animation:{cls}k {T}s linear infinite}}"
    )
    el = (f'<rect class="{cls}" x="{X0 - 12}" y="{y}" width="{W - 2 * X0 + 24}" height="{h}" rx="8" fill="#161b22" stroke="#30363d"/>', css)
    els.insert(len(els) if at is None else at, el)


def P(text, color="fg", flag=None):
    return (text, color, flag)


# ---------------------------------------------------------------- scene 1
s1, e1 = 0.2, 7.2
y = 76
line(y, [P("$ ", "prompt"), P("ghostwriter add ~/writing/")], 0.3, e1, 1.0); y += LH
line(y, [P("  added 6 samples · answer, email, essay, report", "dim")], 1.5, e1); y += LH * 1.5
line(y, [P("$ ", "prompt"), P("ghostwriter analyze")], 2.2, e1, 0.7); y += LH
line(y, [P("  your own samples score ", "dim"), P("~80", "green"), P(" · generic AI drafts ", "dim"), P("~23", "red")], 3.1, e1); y += LH
line(y, [P("  → pass mark ", "dim"), P("77", "fg")], 3.5, e1); y += LH * 1.5
line(y, [P("$ ", "prompt"), P("ghostwriter install")], 4.3, e1, 0.7); y += LH
line(y, [P("  installed skill ", "dim"), P("ghostwriter", "blue"), P(" in .agents/skills/", "dim")], 5.2, e1)

# ---------------------------------------------------------------- scene 2
s2, e2 = 7.6, 17.2
y = 76
line(y, [P("you › ", "purple"), P("write a reply to Marta as me")], s2, e2, 1.1); y += LH
line(y, [P("agent: drafting draft.md, pass 1", "dim")], s2 + 1.4, e2); y += 10
draft1 = [
    [P("Hi Marta,", "draft")],
    [],
    [P("Thank you for reaching out! I'm thrilled to confirm Thursday's demo ", "draft"), P("—", "draft", "F")],
    [P("it's not just a meeting, it's", "draft", "F"), P(" a ", "draft"), P("pivotal moment", "draft", "F"), P(" for the project.", "draft")],
    [],
    [P("I will ensure the slides are ready by Wednesday", "draft"), P(", highlighting", "draft", "F"), P(" our ", "draft"), P("robust", "draft", "F")],
    [P("progress. The payment API sandbox is down, and ", "draft"), P("47", "draft", "F"), P(" tickets are blocked.", "draft")],
    [],
    [P("Let me know if you'd like me", "draft", "F"), P(" to prepare anything else!", "draft")],
]
flag_time = s2 + 4.7
box_top, box_at = y, len(els)
dy = y + 22
for i, segs in enumerate(draft1):
    if segs:
        segs = [(t, c, flag_time if f else None) for t, c, f in segs]
        line(dy, segs, s2 + 1.7 + i * 0.07, e2, size=13)
        dy += 19
    else:
        dy += 9
box(box_top, dy - box_top - 6, s2 + 1.6, e2, at=box_at)
y = dy + 18
line(y, [P("$ ", "prompt"), P("ghostwriter score draft.md -g email --source brief.md")], s2 + 3.0, e2, 1.3); y += LH * 1.4
line(y, [P("Voice match ", "fg"), P("47.8", "red"), P("/100", "dim"), P("  ✗ needs work", "red")], s2 + 4.7, e2, weight="bold"); y += LH
line(y, [P("resemblance 76.8 − AI tells 24.0 − facts 5.0", "dim")], s2 + 5.0, e2); y += LH * 1.2
line(y, [P("  ✗ ", "red"), P("L4  ", "dim"), P('"not X but Y" contrast  ', "yellow"), P("state the point directly", "dim")], s2 + 5.4, e2); y += LH
line(y, [P("  ✗ ", "red"), P("L9  ", "dim"), P("assistant wrapper       ", "yellow"), P("cut the offer, keep the content", "dim")], s2 + 5.6, e2); y += LH
line(y, [P("  ✗ ", "red"), P("L7  ", "dim"), P('invented number "47"    ', "yellow"), P("not in the brief", "dim")], s2 + 5.8, e2)

# ---------------------------------------------------------------- scene 3
s3, e3 = 17.6, 26.6
y = 76
line(y, [P("agent: revising from the notes, pass 2", "dim")], s3, e3, final=True); y += 10
draft2 = [
    "Hey Marta,",
    "",
    "Thursday works, I'll be there. Slides will be ready by Wednesday, nothing",
    "fancy, just the numbers and a couple of screenshots.",
    "",
    "One thing though, the payment API sandbox is still down, so honestly I",
    "can't show that part live. I'd rather tell them now than find out during",
    "the demo haha.",
    "",
    "Thanks a lot!",
]
box_top, box_at = y, len(els)
dy = y + 22
for i, text in enumerate(draft2):
    if text:
        line(dy, [P(text, "draft")], s3 + 0.4 + i * 0.07, e3, size=13, final=True)
        dy += 19
    else:
        dy += 9
box(box_top, dy - box_top - 6, s3 + 0.3, e3, final=True, at=box_at)
y = dy + 18
line(y, [P("$ ", "prompt"), P("ghostwriter score draft.md -g email --source brief.md")], s3 + 1.6, e3, 1.3, final=True); y += LH * 1.4
line(y, [P("Voice match ", "fg"), P("82.5", "green"), P("/100", "dim"), P("  ✓ pass", "green")], s3 + 3.2, e3, final=True, weight="bold"); y += LH
line(y, [P("resemblance 82.5 − AI tells 0.0 − facts 0.0", "dim")], s3 + 3.5, e3, final=True); y += LH * 1.4
line(y, [P("→ ", "green"), P("delivered in your voice, nothing invented", "fg")], s3 + 4.2, e3, final=True)

# ---------------------------------------------------------------- captions
caps = [
    (s1, e1, "1", "learn your voice from your own writing"),
    (s2, e2, "2", "score the agent's draft: voice, AI tells, facts"),
    (s3, e3, "3", "revise until it passes"),
]
for start, end, num, text in caps:
    n += 1
    cls = f"e{n}"
    css = (
        f"@keyframes {cls}k{{0%,{pct(start)}{{opacity:0}}{pct(start + 0.3)},{pct(end)}{{opacity:1}}"
        f"{pct(end + 0.25)},100%{{opacity:0}}}}"
        f".{cls}{{opacity:{1 if num == '3' else 0};animation:{cls}k {T}s linear infinite}}"
    )
    svg = (
        f'<g class="{cls}"><circle cx="{X0 + 9}" cy="{H - 34}" r="10" fill="#1f6feb"/>'
        f'<text x="{X0 + 9}" y="{H - 30}" font-size="12" fill="#fff" text-anchor="middle" font-weight="bold">{num}</text>'
        f'<text x="{X0 + 28}" y="{H - 29.5}" font-size="13" fill="{C["dim"]}">{escape(text)}</text></g>'
    )
    els.append((svg, css))

# step dots on the right
for i, (start, end, num, _) in enumerate(caps):
    n += 1
    cls = f"e{n}"
    css = (
        f"@keyframes {cls}k{{0%,{pct(start)}{{fill:#30363d}}{pct(start + 0.3)},{pct(end)}{{fill:#1f6feb}}"
        f"{pct(end + 0.25)},100%{{fill:#30363d}}}}"
        f".{cls}{{fill:{'#1f6feb' if num == '3' else '#30363d'};animation:{cls}k {T}s linear infinite}}"
    )
    els.append((f'<rect class="{cls}" x="{W - 120 + i * 26}" y="{H - 37}" width="20" height="6" rx="3"/>', css))

style = "".join(c for _, c in els)
body = "\n".join(s for s, _ in els)
svg = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="ghostwriter learns a writer's voice, flags an AI-sounding draft at 47.8/100, and passes the revised draft at 82.5/100">
<title>ghostwriter: learn your voice, score the draft, revise until it passes</title>
<style>
text{{font-family:{FONT};}}
{style}
@media (prefers-reduced-motion:reduce){{*{{animation:none!important}}}}
</style>
<rect x="8" y="8" width="{W - 16}" height="{H - 16}" rx="12" fill="#0d1117" stroke="#30363d"/>
<path d="M8 20a12 12 0 0 1 12-12h{W - 40}a12 12 0 0 1 12 12v22H8z" fill="#161b22"/>
<line x1="8" y1="42" x2="{W - 8}" y2="42" stroke="#30363d"/>
<circle cx="30" cy="25" r="6" fill="#ff5f57"/><circle cx="50" cy="25" r="6" fill="#febc2e"/><circle cx="70" cy="25" r="6" fill="#28c840"/>
<text x="{W / 2}" y="30" font-size="13" fill="{C['dim']}" text-anchor="middle">ghostwriter</text>
<line x1="8" y1="{H - 58}" x2="{W - 8}" y2="{H - 58}" stroke="#21262d"/>
{body}
</svg>
"""
open(sys.argv[1], "w", encoding="utf-8").write(svg)
print(f"{len(svg)} bytes, {n} animated elements")
