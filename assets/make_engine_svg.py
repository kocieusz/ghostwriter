"""Generate assets/engine.svg: an animated diagram of what ghostwriter computes.

    python3 assets/make_engine_svg.py profile.json draft1.json draft2.json assets/engine.svg

Inputs are real ghostwriter output, made the same way as assets/demo.svg:
a profile built from internal/voice/testdata/writer, then

    ghostwriter analyze --json                                        > profile.json
    ghostwriter score pass1.md -g email --source brief.md --json      > draft1.json
    ghostwriter score pass2.md -g email --source brief.md --json      > draft2.json

where pass1.md and pass2.md are the two drafts shown in assets/demo.svg.
Bands, dots, tell counts, calibration points and scores all come from those
files; nothing is drawn from made-up numbers.
"""
import json
import math
import sys
from html import escape

prof, d1, d2 = (json.load(open(p)) for p in sys.argv[1:4])
OUT = sys.argv[4]

T = 28.0
W, H = 880, 560
FONT = "ui-monospace,SFMono-Regular,Menlo,Consolas,'Liberation Mono',monospace"
COL = {
    "bg": "#0d1117", "panel": "#161b22", "line": "#30363d", "dim": "#8b949e", "fg": "#e6edf3",
    "blue": "#1f6feb", "lblue": "#79c0ff", "green": "#3fb950", "amber": "#d29922", "red": "#f85149",
    "purple": "#d2a8ff",
}

css, svg = [], []
uid = 0


def pct(t):
    return f"{max(0.0, min(100.0, t / T * 100)):.3f}%"


def anim(frames, final):
    """frames: list of (time, css-declarations). final: declarations used when
    the animation does not run (reduced motion, static renderers)."""
    global uid
    uid += 1
    name = f"a{uid}"
    stops = []
    for t, decl in frames:
        stops.append(f"{pct(t)}{{{decl}}}")
    css.append(f"@keyframes {name}{{{''.join(stops)}}}.{name}{{{final};animation:{name} {T}s linear infinite}}")
    return name


def show(start, end, final=False, fade=0.3):
    """Fade in at start, out at end."""
    return anim([(0, "opacity:0"), (start, "opacity:0"), (start + fade, "opacity:1"),
                 (end, "opacity:1"), (end + fade, "opacity:0"), (T, "opacity:0")],
                f"opacity:{1 if final else 0}")


def text(x, y, s, size=12, fill="dim", anchor="start", weight=None, cls=None):
    w = f' font-weight="{weight}"' if weight else ""
    c = f' class="{cls}"' if cls else ""
    return f'<text x="{x}" y="{y}" font-size="{size}" fill="{COL.get(fill, fill)}" text-anchor="{anchor}"{w}{c}>{escape(s)}</text>'


# ---------------------------------------------------------------- timeline
LEARN, CAL, SCAN, REVISE, END = 0.4, 6.4, 10.4, 17.6, 26.6

# ---------------------------------------------------------------- frame
svg.append(f'<rect x="4" y="4" width="{W - 8}" height="{H - 8}" rx="14" fill="{COL["bg"]}" stroke="{COL["line"]}"/>')
svg.append(f'<pattern id="dots" width="22" height="22" patternUnits="userSpaceOnUse"><circle cx="1" cy="1" r="1" fill="#161b22"/></pattern>')
svg.append(f'<rect x="4" y="4" width="{W - 8}" height="{H - 8}" rx="14" fill="url(#dots)"/>')

# phase indicator
phases = [("learn", LEARN, CAL), ("calibrate", CAL, SCAN), ("scan a draft", SCAN, REVISE), ("revise", REVISE, END)]
px = 30
for i, (label, a, b) in enumerate(phases):
    on = anim([(0, f"fill:{COL['dim']}"), (a, f"fill:{COL['dim']}"), (a + 0.3, f"fill:{COL['fg']}"),
               (b, f"fill:{COL['fg']}"), (b + 0.3, f"fill:{COL['dim']}"), (T, f"fill:{COL['dim']}")],
              f"fill:{COL['fg'] if i == 3 else COL['dim']}")
    pill = anim([(0, f"fill:{COL['line']}"), (a, f"fill:{COL['line']}"), (a + 0.3, f"fill:{COL['blue']}"),
                 (b, f"fill:{COL['blue']}"), (b + 0.3, f"fill:{COL['line']}"), (T, f"fill:{COL['line']}")],
                f"fill:{COL['blue'] if i == 3 else COL['line']}")
    svg.append(f'<circle class="{pill}" cx="{px + 6}" cy="27" r="6"/>')
    svg.append(f'<text class="{on}" x="{px + 17}" y="31" font-size="12">{i + 1} {escape(label)}</text>')
    px += 30 + len(label) * 7.6 + 14
svg.append(text(W - 30, 31, "ghostwriter · how a draft is judged", 12, "dim", "end"))
svg.append(f'<line x1="4" y1="46" x2="{W - 4}" y2="46" stroke="{COL["line"]}"/>')

# ---------------------------------------------------------------- corpus
CX, CW = 26, 158
svg.append(text(CX, 70, "YOUR WRITING", 11, "dim", weight="bold"))
docs = prof["docs"]
doc_y = []
for i, d in enumerate(docs):
    y = 80 + i * 50
    doc_y.append(y + 20)
    lit = anim([(0, f"stroke:{COL['line']}"), (LEARN + 0.4 + i * 0.5, f"stroke:{COL['line']}"),
                (LEARN + 0.6 + i * 0.5, f"stroke:{COL['lblue']}"), (LEARN + 1.4 + i * 0.5, f"stroke:{COL['line']}"),
                (T, f"stroke:{COL['line']}")], f"stroke:{COL['line']}")
    svg.append(f'<rect class="{lit}" x="{CX}" y="{y}" width="{CW}" height="40" rx="6" fill="{COL["panel"]}"/>')
    for k, wfrac in enumerate((0.55, 0.8, 0.4)):
        svg.append(f'<rect x="{CX + 10}" y="{y + 9 + k * 8}" width="{22 * wfrac}" height="3" rx="1.5" fill="#484f58"/>')
    svg.append(text(CX + 40, y + 17, d["name"].rsplit(".", 1)[0], 11, "fg"))
    svg.append(text(CX + 40, y + 31, f'{d["genre"]} · {d["words"]} words', 10, "dim"))

# ---------------------------------------------------------------- fingerprint
FX, FW_ = 212, 432
TX0, TX1 = 392, 628  # track
svg.append(f'<rect x="{FX}" y="56" width="{FW_}" height="356" rx="10" fill="{COL["panel"]}" stroke="{COL["line"]}"/>')
svg.append(text(FX + 14, 76, "FINGERPRINT", 11, "dim", weight="bold"))
svg.append(text(FX + FW_ - 14, 76, "your range per measurement", 10, "dim", "end"))

BANDED = {"discourse_marker_rate", "pct_para_initial_connective", "pct_sent_start_conjunction", "comma_per_sentence",
          "pct_run_on_sentences", "first_person_rate", "i_think_rate", "hedge_rate", "intensifier_rate",
          "signature_rate", "lowercase_i_rate", "cap_anomaly_rate"}
ROWS = [
    ("mean_sentence_len", "sentence length"),
    ("cv_sentence_len", "rhythm variation"),
    ("mean_word_len", "word length"),
    ("discourse_marker_rate", "connectives"),
    ("comma_per_sentence", "commas / sentence"),
    ("first_person_rate", "first person"),
    ("contraction_rate", "contractions"),
    ("signature_rate", "signature phrases"),
]


def sigma(name, mean, std):
    """Same clamp as voice.clampSigma."""
    floor = 0.15 * abs(mean)
    if name.endswith("_rate"):
        floor = max(floor, 1.5)
    elif name.startswith("pct_"):
        floor = max(floor, 4)
    elif name in ("mean_sentence_len", "std_sentence_len"):
        floor = max(floor, 1.5)
    else:
        floor = max(floor, 0.02)
    ceil = max(0.6 * abs(mean), floor)
    return min(max(std, floor), ceil)


def tol(res):
    # voice.Score widens the tolerance for drafts shorter than a typical sample
    return 1.6 * min(2, max(1, math.sqrt(prof["median_words"] / max(res["words"], 1))))


def color(score):
    return COL["green"] if score >= 0.8 else COL["amber"] if score >= 0.5 else COL["red"]


tolerance = (tol(d1) + tol(d2)) / 2
k = tolerance * math.sqrt(-2 * math.log(0.8))  # |z| where a feature still scores 0.8
mismatch = []
for i, (name, label) in enumerate(ROWS):
    y = 102 + i * 34
    st = prof["features"][name]
    mean = d1["features"][name]["mean"]  # genre-blended mean the scorer used
    sg = sigma(name, mean, st["std"])
    lo_b = mean - k * sg
    hi_b = mean + (2 + k) * sg if name in BANDED else mean + k * sg
    v1, v2 = d1["features"][name]["draft"], d2["features"][name]["draft"]
    top = max(hi_b, v1, v2, mean * 1.6) * 1.12
    sx = lambda v: TX0 + (TX1 - TX0) * max(0.0, min(1.0, v / top))
    for v, res in ((v1, d1), (v2, d2)):
        inside = lo_b <= v <= hi_b
        if inside != (res["features"][name]["score"] >= 0.8):
            mismatch.append((name, v, res["features"][name]["score"]))

    svg.append(text(FX + 16, y + 4, label, 12, "fg"))
    svg.append(f'<line x1="{TX0}" y1="{y}" x2="{TX1}" y2="{y}" stroke="{COL["line"]}" stroke-width="2" stroke-linecap="round"/>')
    grow = anim([(0, "opacity:0;transform:scaleX(0)"), (LEARN + 1.2 + i * 0.4, "opacity:0;transform:scaleX(0)"),
                 (LEARN + 2.0 + i * 0.4, "opacity:1;transform:scaleX(1)"), (END, "opacity:1;transform:scaleX(1)"),
                 (END + 0.4, "opacity:0;transform:scaleX(1)"), (T, "opacity:0;transform:scaleX(0)")],
                "opacity:1;transform:scaleX(1)")
    bx0, bx1 = sx(max(lo_b, 0)), sx(hi_b)
    svg.append(f'<rect class="{grow} band" x="{bx0:.1f}" y="{y - 7}" width="{max(bx1 - bx0, 3):.1f}" height="14" rx="7" fill="{COL["blue"]}" fill-opacity="0.32" stroke="{COL["blue"]}" stroke-opacity="0.6"/>')
    tick = show(LEARN + 1.8 + i * 0.4, END, final=True)
    svg.append(f'<line class="{tick}" x1="{sx(mean):.1f}" y1="{y - 9}" x2="{sx(mean):.1f}" y2="{y + 9}" stroke="{COL["lblue"]}" stroke-width="2"/>')

    # the draft's value: lands at pass 1, slides to pass 2
    s1, s2 = d1["features"][name]["score"], d2["features"][name]["score"]
    x1, x2 = sx(v1), sx(v2)
    t_in = SCAN + 2.4 + i * 0.18
    t_mv = REVISE + 0.8 + i * 0.18
    dot = anim([(0, f"opacity:0;transform:translateX(0);fill:{color(s1)}"),
                (t_in, f"opacity:0;transform:translateX(0);fill:{color(s1)}"),
                (t_in + 0.3, f"opacity:1;transform:translateX(0);fill:{color(s1)}"),
                (t_mv, f"opacity:1;transform:translateX(0);fill:{color(s1)}"),
                (t_mv + 0.9, f"opacity:1;transform:translateX({x2 - x1:.1f}px);fill:{color(s2)}"),
                (END, f"opacity:1;transform:translateX({x2 - x1:.1f}px);fill:{color(s2)}"),
                (END + 0.4, f"opacity:0;transform:translateX({x2 - x1:.1f}px);fill:{color(s2)}"),
                (T, f"opacity:0;transform:translateX(0);fill:{color(s1)}")],
               f"opacity:1;transform:translateX({x2 - x1:.1f}px);fill:{color(s2)}")
    svg.append(f'<circle class="{dot}" cx="{x1:.1f}" cy="{y}" r="5.5" stroke="{COL["bg"]}" stroke-width="2"/>')

n_named = len([f for f in prof["features"] if not f.startswith("fw_")])
n_fw = len([f for f in prof["features"] if f.startswith("fw_")])
foot_y = 102 + len(ROWS) * 34 - 6
svg.append(text(FX + 16, foot_y, f"+ {n_named - len(ROWS)} more measurements · {n_fw}-word function-word profile", 10, "dim"))
# signature phrases
chips = [s for s in prof["signatures"] if len(s) <= 14][:4]
svg.append(text(FX + 16, foot_y + 22, "signature phrases", 10, "dim"))
cx = FX + 132
for j, s in enumerate(chips):
    w = len(s) * 7 + 16
    if cx + w > FX + FW_ - 14:
        break  # only as many as fit inside the panel
    pop = show(LEARN + 4.2 + j * 0.25, END, final=True)
    svg.append(f'<g class="{pop}"><rect x="{cx}" y="{foot_y + 10}" width="{w}" height="18" rx="9" fill="#1f2937" stroke="{COL["purple"]}" stroke-opacity="0.6"/>'
               + text(cx + w / 2, foot_y + 23, s, 10.5, "purple", "middle") + "</g>")
    cx += w + 8

# particles: samples stream into the fingerprint
pid = 0
for i, y0 in enumerate(doc_y):
    for j in range(3):
        row = (i * 3 + j * 5) % len(ROWS)
        tx = TX0 + 20 + ((i * 37 + j * 53) % 180)
        ty = 102 + row * 34
        start = LEARN + 0.5 + i * 0.5 + j * 0.22
        dx, dy = tx - (CX + CW), ty - y0
        p = anim([(0, "opacity:0;transform:translate(0,0)"), (start, "opacity:0;transform:translate(0,0)"),
                  (start + 0.1, "opacity:1;transform:translate(0,0)"),
                  (start + 0.9, f"opacity:0.9;transform:translate({dx}px,{dy}px)"),
                  (start + 1.1, f"opacity:0;transform:translate({dx}px,{dy}px)"), (T, "opacity:0;transform:translate(0,0)")],
                 "opacity:0")
        svg.append(f'<circle class="{p}" cx="{CX + CW}" cy="{y0}" r="2.6" fill="{COL["lblue"]}"/>')
        pid += 1

# ---------------------------------------------------------------- draft + scanner
DX, DW = 664, 190
svg.append(text(DX, 70, "DRAFT", 11, "dim", weight="bold"))
card = show(SCAN, END, final=True)
svg.append(f'<g class="{card}">')
svg.append(f'<rect x="{DX}" y="80" width="{DW}" height="168" rx="8" fill="{COL["panel"]}" stroke="{COL["line"]}"/>')
p1 = show(SCAN, REVISE)
p2 = show(REVISE, END, final=True)
svg.append(f'<g class="{p1}">' + text(DX + 12, 99, "draft.md · pass 1", 11, "fg") + "</g>")
svg.append(f'<g class="{p2}">' + text(DX + 12, 99, "draft.md · pass 2", 11, "fg") + "</g>")
widths = [0.45, 0, 0.92, 0.88, 0, 0.95, 0.9, 0, 0.7]
yy = 112
flags = {2: [(0.86, 0.06)], 3: [(0.0, 0.42), (0.55, 0.3)], 5: [(0.62, 0.22), (0.86, 0.12)], 6: [(0.55, 0.06)], 8: [(0.0, 0.5)]}
for li, w in enumerate(widths):
    if w == 0:
        yy += 6
        continue
    svg.append(f'<rect x="{DX + 12}" y="{yy}" width="{(DW - 24) * w:.1f}" height="5" rx="2.5" fill="#484f58"/>')
    for off, fw_ in flags.get(li, []):
        hl = anim([(0, "opacity:0"), (SCAN + 0.8 + li * 0.17, "opacity:0"), (SCAN + 1.0 + li * 0.17, "opacity:1"),
                   (REVISE + 0.4, "opacity:1"), (REVISE + 0.9, "opacity:0"), (T, "opacity:0")], "opacity:0")
        svg.append(f'<rect class="{hl}" x="{DX + 12 + (DW - 24) * w * off:.1f}" y="{yy - 1}" width="{(DW - 24) * w * fw_:.1f}" height="7" rx="3" fill="{COL["red"]}"/>')
    yy += 14
# scanner beam
beam = anim([(0, "opacity:0;transform:translateY(0)"), (SCAN + 0.6, "opacity:0;transform:translateY(0)"),
             (SCAN + 0.7, "opacity:1;transform:translateY(0)"), (SCAN + 2.4, "opacity:1;transform:translateY(126px)"),
             (SCAN + 2.6, "opacity:0;transform:translateY(126px)"),
             (REVISE + 0.2, "opacity:0;transform:translateY(0)"), (REVISE + 0.3, "opacity:1;transform:translateY(0)"),
             (REVISE + 2.0, "opacity:1;transform:translateY(126px)"), (REVISE + 2.2, "opacity:0;transform:translateY(126px)"),
             (T, "opacity:0;transform:translateY(0)")], "opacity:0")
svg.append(f'<rect class="{beam}" x="{DX + 4}" y="108" width="{DW - 8}" height="3" rx="1.5" fill="{COL["lblue"]}" style="filter:drop-shadow(0 0 4px {COL["lblue"]})"/>')
svg.append("</g>")

# tell counter: found vs what this writer's own baseline allows
svg.append(text(DX, 272, "AI PATTERNS", 11, "dim", weight="bold"))
svg.append(text(DX + DW, 272, "found / you use", 10, "dim", "end"))
by_rule = {f["rule"]: f for f in d1["tells"]["findings"]}
by_rule2 = {f["rule"]: f for f in d2["tells"]["findings"]}
rows = [("negative_parallelism", '"not X but Y"'), ("chatbot_residue", "assistant wrapper"),
        ("ai_vocabulary", "AI vocabulary"), ("em_dash", "em dash")]
for i, (rule, label) in enumerate(rows):
    y = 294 + i * 22
    f1, f2 = by_rule.get(rule), by_rule2.get(rule)
    c1 = f1["count"] if f1 else 0
    c2 = f2["count"] if f2 else 0
    allowed = f1["allowed"] if f1 else 0
    bad1 = bool(f1 and f1["penalty"] > 0)
    show_row = show(SCAN + 2.0 + i * 0.25, END, final=True)
    svg.append(f'<g class="{show_row}">' + text(DX, y, label, 11, "fg"))
    r1 = show(SCAN + 2.0 + i * 0.25, REVISE + 1.2)
    r2 = show(REVISE + 1.2, END, final=True)
    svg.append(f'<g class="{r1}">' + text(DX + DW, y, f"{c1} / {allowed:g}", 11, "red" if bad1 else "green", "end", "bold") + "</g>")
    svg.append(f'<g class="{r2}">' + text(DX + DW, y, f"{c2} / {allowed:g}", 11, "green", "end", "bold") + "</g>")
    svg.append("</g>")
note = show(SCAN + 3.2, END, final=True)
svg.append(f'<g class="{note}">' + text(DX, 384, "this writer uses dashes,", 10, "dim") + text(DX, 398, "so one is allowed", 10, "dim") + "</g>")

# ---------------------------------------------------------------- calibration axis
AX0, AX1, AY = 236, 850, 478
svg.append(text(26, 440, "CALIBRATION", 11, "dim", weight="bold"))
svg.append(f'<line x1="{AX0}" y1="{AY}" x2="{AX1}" y2="{AY}" stroke="{COL["line"]}" stroke-width="2"/>')
ax = lambda v: AX0 + (AX1 - AX0) * v / 100
for v in (0, 25, 50, 75, 100):
    svg.append(f'<line x1="{ax(v)}" y1="{AY - 4}" x2="{ax(v)}" y2="{AY + 4}" stroke="{COL["line"]}"/>')
    svg.append(text(ax(v), AY + 18, str(v), 10, "dim", "middle"))
cal = prof["calibration"]
for i, s in enumerate(cal["samples"]):
    start = CAL + 0.3 + i * 0.22
    c = COL["red"] if s.get("ai") else COL["green"]
    drop = anim([(0, "opacity:0;transform:translateY(-26px)"), (start, "opacity:0;transform:translateY(-26px)"),
                 (start + 0.45, "opacity:1;transform:translateY(0)"), (END, "opacity:1;transform:translateY(0)"),
                 (END + 0.4, "opacity:0;transform:translateY(0)"), (T, "opacity:0;transform:translateY(-26px)")],
                "opacity:1;transform:translateY(0)")
    yoff = -7 if (i % 2) else 7
    svg.append(f'<circle class="{drop}" cx="{ax(s["overall"]):.1f}" cy="{AY + (0 if not s.get("ai") else 0)}" r="5" fill="{c}" fill-opacity="0.85"/>')
legend = show(CAL + 1.2, END, final=True)
svg.append(f'<g class="{legend}"><circle cx="40" cy="462" r="4.5" fill="{COL["green"]}"/>' + text(52, 466, "your samples, held out", 11, "fg")
           + f'<circle cx="40" cy="484" r="4.5" fill="{COL["red"]}"/>' + text(52, 488, "generic AI drafts", 11, "fg") + "</g>")
pm = cal["target"]
mark = anim([(0, "opacity:0;transform:scaleY(0)"), (CAL + 2.6, "opacity:0;transform:scaleY(0)"),
             (CAL + 3.2, "opacity:1;transform:scaleY(1)"), (END, "opacity:1;transform:scaleY(1)"),
             (END + 0.4, "opacity:0;transform:scaleY(1)"), (T, "opacity:0;transform:scaleY(0)")],
            "opacity:1;transform:scaleY(1)")
svg.append(f'<g class="{mark}" style="transform-origin:{ax(pm):.1f}px {AY}px"><line x1="{ax(pm):.1f}" y1="{AY - 40}" x2="{ax(pm):.1f}" y2="{AY + 8}" stroke="{COL["fg"]}" stroke-dasharray="4 3"/>'
           + text(ax(pm), AY - 46, f"pass mark {pm:g}", 11, "fg", "middle", "bold") + "</g>")

# the draft's score: lands at pass 1, moves at pass 2
o1, o2 = d1["overall"], d2["overall"]
sx1, sx2 = ax(o1), ax(o2)
sc = anim([(0, f"opacity:0;transform:translate(0,-60px);fill:{COL['red']}"),
           (SCAN + 4.6, f"opacity:0;transform:translate(0,-60px);fill:{COL['red']}"),
           (SCAN + 5.3, f"opacity:1;transform:translate(0,0);fill:{COL['red']}"),
           (REVISE + 3.2, f"opacity:1;transform:translate(0,0);fill:{COL['red']}"),
           (REVISE + 4.4, f"opacity:1;transform:translate({sx2 - sx1:.1f}px,0);fill:{COL['green']}"),
           (END, f"opacity:1;transform:translate({sx2 - sx1:.1f}px,0);fill:{COL['green']}"),
           (END + 0.4, f"opacity:0;transform:translate({sx2 - sx1:.1f}px,0);fill:{COL['green']}"),
           (T, f"opacity:0;transform:translate(0,-60px);fill:{COL['red']}")],
          f"opacity:1;transform:translate({sx2 - sx1:.1f}px,0);fill:{COL['green']}")
svg.append(f'<path class="{sc}" d="M{sx1:.1f} {AY - 11}l9 11-9 11-9-11z" stroke="{COL["bg"]}" stroke-width="2"/>')
l1 = show(SCAN + 5.3, REVISE + 3.2)
l2 = show(REVISE + 4.4, END, final=True)
svg.append(f'<g class="{l1}">' + text(sx1, AY + 36, f"draft {o1:g} ✗", 12, "red", "middle", "bold") + "</g>")
svg.append(f'<g class="{l2}">' + text(sx2, AY + 36, f"draft {o2:g} ✓", 12, "green", "middle", "bold") + "</g>")

# score equation
def eq(res, cls, ok):
    return (f'<g class="{cls}">'
            + text(26, 516, f"{res['resemblance']:g} voice − {res['tell_penalty']:g} tells − {res['fact_penalty']:g} facts", 11, "dim")
            + text(26, 536, f"= {res['overall']:g}  {'pass' if ok else 'below the mark'}", 12, "green" if ok else "red", weight="bold")
            + "</g>")
svg.append(eq(d1, show(SCAN + 5.3, REVISE + 3.2), d1["pass"]))
svg.append(eq(d2, show(REVISE + 4.4, END, final=True), d2["pass"]))

if mismatch:
    print("warning: band/score colour mismatch:", mismatch, file=sys.stderr)

doc = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="ghostwriter learns a fingerprint from a writer's samples, calibrates a pass mark at {pm:g}, scans a draft for AI patterns against the writer's own baseline, and scores it {o1:g}; the revision scores {o2:g} and passes">
<title>How ghostwriter judges a draft: fingerprint, calibration, AI-pattern scan, score</title>
<style>
text{{font-family:{FONT};}}
.band{{transform-box:fill-box;transform-origin:center}}
circle,path{{transform-box:view-box}}
{''.join(css)}
@media (prefers-reduced-motion:reduce){{*{{animation:none!important}}}}
</style>
{chr(10).join(svg)}
</svg>
"""
open(OUT, "w", encoding="utf-8").write(doc)
print(f"{len(doc)} bytes, {uid} animations")
