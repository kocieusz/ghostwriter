"""Generate assets/engine.svg: a tmux session showing what ghostwriter computes.

    python3 assets/make_engine_svg.py profile.json draft1.json draft2.json assets/engine.svg

Inputs are real ghostwriter output, made the same way as assets/demo.svg:
a profile built from internal/voice/testdata/writer, then

    ghostwriter analyze --json                                    > profile.json
    ghostwriter score pass1.md -g email --source brief.md --json  > draft1.json
    ghostwriter score pass2.md -g email --source brief.md --json  > draft2.json

where pass1.md and pass2.md are the two drafts shown in assets/demo.svg
(PASS1 and PASS2 below, re-wrapped to the pane width). Ranges, draft values,
pattern counts, calibration points and scores all come from those files.

The animation is a sequence of terminal frames. Each pane row becomes one
<text> element per stretch of time it stays unchanged, switched on and off
with step timing, so it redraws like a terminal instead of fading.
"""
import json
import math
import sys
from html import escape

prof, d1, d2 = (json.load(open(p)) for p in sys.argv[1:4])
OUT = sys.argv[4]

T = 30.0
CW, LH, FS = 7.8, 18, 13          # cell width, line height, font size
PAD, TOP = 18, 50                  # left padding, baseline of the first row
LEFT_W, RIGHT_X = 50, 51           # left pane width; right pane first column
ROWS = 26
W = int(PAD * 2 + 100 * CW)
STATUS_Y = TOP + ROWS * LH + 6
H = STATUS_Y + 14

# Tokyo Night
C = dict(bg="#1a1b26", bar="#16161e", fg="#c0caf5", dim="#565f89", blue="#7aa2f7", cyan="#7dcfff",
         green="#9ece6a", red="#f7768e", yellow="#e0af68", magenta="#bb9af7", border="#3b4261")

PASS1 = """Hi Marta,
Thank you for reaching out! I'm thrilled to
confirm Thursday's demo — it's not just a
meeting, it's a pivotal moment for the
project. I will ensure the slides are ready
by Wednesday, highlighting our robust
progress. The payment API sandbox is down,
and 47 tickets are blocked.
Let me know if you'd like me to prepare
anything else!""".split("\n")
FLAGS = ["—", "it's not just a", "meeting, it's", "pivotal moment", ", highlighting", "robust", "47",
         "Let me know if you'd like me"]
PASS2 = """Hey Marta,
Thursday works, I'll be there. Slides will be
ready by Wednesday, nothing fancy, just the
numbers and a couple of screenshots.
One thing though, the payment API sandbox is
still down, so honestly I can't show that
part live. I'd rather tell them now than
find out during the demo haha.
Thanks a lot!""".split("\n")

L, R = "L", "R"

# ---------------------------------------------------------------- frames
# Events change rows of a cumulative screen: {(pane, row): [(text, colour)]}.
events = []


def at(t, changes):
    events.append((t, changes))


def runs(cells, indent="  "):
    """Merge (char, colour) cells into (text, colour) runs."""
    out = [(indent, "fg")]
    for ch, c in cells:
        if len(out) > 1 and out[-1][1] == c:
            out[-1] = (out[-1][0] + ch, c)
        else:
            out.append((ch, c))
    return out


def typed(t0, pane, row, cmd, per=0.06):
    for i in range(len(cmd) + 1):
        at(t0 + i * per, {(pane, row): [("$ ", "green"), (cmd[:i], "fg"), ("█", "fg")]})
    end = t0 + (len(cmd) + 1) * per + 0.15
    at(end, {(pane, row): [("$ ", "green"), (cmd, "fg")]})
    return end


# ---- measurements: same spread clamp and tolerance as voice.Score
BANDED = {"discourse_marker_rate", "pct_para_initial_connective", "pct_sent_start_conjunction", "comma_per_sentence",
          "pct_run_on_sentences", "first_person_rate", "i_think_rate", "hedge_rate", "intensifier_rate",
          "signature_rate", "lowercase_i_rate", "cap_anomaly_rate"}
FEATS = [("mean_sentence_len", "sentence length"), ("cv_sentence_len", "rhythm"), ("mean_word_len", "word length"),
         ("discourse_marker_rate", "connectives"), ("comma_per_sentence", "commas/sentence"),
         ("first_person_rate", "first person"), ("contraction_rate", "contractions"),
         ("signature_rate", "signature phrases")]
TRACK = 22


def sigma(name, mean, std):
    floor = 0.15 * abs(mean)
    if name.endswith("_rate"):
        floor = max(floor, 1.5)
    elif name.startswith("pct_"):
        floor = max(floor, 4)
    elif name in ("mean_sentence_len", "std_sentence_len"):
        floor = max(floor, 1.5)
    else:
        floor = max(floor, 0.02)
    return min(max(std, floor), max(0.6 * abs(mean), floor))


tolerance = sum(1.6 * min(2, max(1, math.sqrt(prof["median_words"] / max(r["words"], 1)))) for r in (d1, d2)) / 2
K = tolerance * math.sqrt(-2 * math.log(0.8))   # |z| at which a measurement still scores 0.8
geom, problems = [], []
for name, label in FEATS:
    mean = d1["features"][name]["mean"]          # the genre-blended mean the scorer used
    sg = sigma(name, mean, prof["features"][name]["std"])
    lo, hi = mean - K * sg, mean + ((2 + K) if name in BANDED else K) * sg
    v1, v2 = d1["features"][name]["draft"], d2["features"][name]["draft"]
    top = max(hi, v1, v2, mean * 1.6) * 1.08
    col = lambda v, top=top: max(0, min(TRACK - 1, round(v / top * (TRACK - 1))))
    geom.append(dict(label=label, lo=col(max(lo, 0)), hi=col(hi), mean=col(mean), v=(v1, v2), c=(col(v1), col(v2)),
                     s=(d1["features"][name]["score"], d2["features"][name]["score"])))
    for v, r in ((v1, d1), (v2, d2)):
        if (lo <= v <= hi) != (r["features"][name]["score"] >= 0.8):
            problems.append((name, v))


def grade(score):
    return "green" if score >= 0.8 else "yellow" if score >= 0.5 else "red"


def feat_row(g, draft=None):
    """label, then ───━━━┃━━─── with the draft's ● on it, then its value."""
    cells = [("─", "dim")] * TRACK
    for c in range(g["lo"], g["hi"] + 1):
        cells[c] = ("━", "blue")
    cells[g["mean"]] = ("┃", "cyan")
    tail = []
    if draft is not None:
        i = draft - 1
        cells[g["c"][i]] = ("●", grade(g["s"][i]))
        v = g["v"][i]
        tail = [(f"{v:6.1f}" if v < 10 else f"{v:6.0f}", grade(g["s"][i]))]
    return [("  " + g["label"].ljust(18), "fg")] + runs(cells, "")[1:] + tail


cal = prof["calibration"]
AXIS = 40   # 0..100 in steps of 2.5


def axis_row(samples, draft=None):
    cells = [("·", "dim")] * (AXIS + 1)
    for s in samples:
        cells[round(s["overall"] / 2.5)] = ("x", "red") if s.get("ai") else ("o", "green")
    if draft is None or len(samples) == len(cal["samples"]):
        if len(samples) == len(cal["samples"]):
            cells[round(cal["target"] / 2.5)] = ("┃", "yellow")
    if draft:
        r = (d1, d2)[draft - 1]
        cells[round(r["overall"] / 2.5)] = ("●", "green" if r["pass"] else "red")
    return runs(cells)


# ---------------------------------------------------------------- script
own = [s["overall"] for s in cal["samples"] if not s.get("ai")]
ai = [s["overall"] for s in cal["samples"] if s.get("ai")]

# phase 0: learn
at(0, {(L, 0): [("$ ", "green"), ("█", "fg")], (R, 0): [("$ ", "green")]})
t = typed(0.6, L, 0, "ghostwriter analyze")
for i, d in enumerate(prof["docs"]):
    at(t + 0.3 + i * 0.2, {(L, 2 + i): [("  " + d["name"].ljust(22), "fg"), (d["genre"].ljust(8), "dim"),
                                       (f'{d["words"]:>4} words', "dim")]})
t += 0.3 + len(prof["docs"]) * 0.2 + 0.3
at(t, {(L, 9): [("  measurement       ", "dim"), ("━ your range ", "blue"), ("┃", "cyan"), (" mean", "dim")]})
for i, g in enumerate(geom):
    at(t + 0.3 + i * 0.22, {(L, 10 + i): feat_row(g)})
t += 0.3 + len(geom) * 0.22 + 0.2
n_named = len([f for f in prof["features"] if not f.startswith("fw_")])
n_fw = len([f for f in prof["features"] if f.startswith("fw_")])
at(t, {(L, 18): [(f"  + {n_named - len(FEATS)} more · {n_fw} function words · {len(prof['tells'])} patterns", "dim")]})
phr = [s for s in prof["signatures"] if len(s) <= 12][:3]
at(t + 0.3, {(L, 19): [("  phrases  ", "dim"), (" · ".join(phr), "magenta")]})

# phase 1: calibrate
CAL_T = 8.4
at(CAL_T, {(L, 21): [("  calibration", "fg"), (" · each sample vs the rest", "dim")],
           (L, 22): [("  0         25        50        75       100", "dim")]})
order = sorted(cal["samples"], key=lambda s: s["overall"])
for i in range(len(order)):
    at(CAL_T + 0.4 + i * 0.2, {(L, 23): axis_row(order[:i + 1])})
t = CAL_T + 0.4 + len(order) * 0.2 + 0.2
at(t, {(L, 24): [("  o", "green"), (f" yours ~{sum(own) / len(own):.0f}  ", "dim"), ("x", "red"),
                 (f" generic AI ~{sum(ai) / len(ai):.0f}  ", "dim"), ("┃", "yellow"), (f" pass {cal['target']:g}", "fg")]})

# phase 2: scan pass 1
SCAN_T = 12.6
t = typed(SCAN_T, R, 0, "ghostwriter score draft.md -s brief.md", per=0.04)
at(t + 0.2, {(R, 2 + i): [("  " + line, "fg")] for i, line in enumerate(PASS1)})


def flagged(line):
    parts, rest = [("  ", "fg")], line
    while rest:
        hits = [(rest.find(f), f) for f in FLAGS if f in rest]
        if not hits:
            parts.append((rest, "fg"))
            break
        idx, f = min(hits)
        if idx:
            parts.append((rest[:idx], "fg"))
        parts.append((f, "red"))
        rest = rest[idx + len(f):]
    return parts


t_scan = t + 0.8
for i, line in enumerate(PASS1):  # the scan reaches one line at a time
    at(t_scan + i * 0.12, {(R, 2 + i): flagged(line)})
t = t_scan + len(PASS1) * 0.12 + 0.3
NAMES = {"negative_parallelism": '"not X but Y"', "chatbot_residue": "assistant wrapper",
         "ai_vocabulary": "AI vocabulary", "inflated_significance": "inflated claim",
         "ing_rider": "-ing rider", "em_dash": "em dash"}
at(t, {(R, 13): [("  pattern              found  yours   pts", "dim")]})
finds = sorted(d1["tells"]["findings"], key=lambda f: -f["penalty"])
for i, f in enumerate(finds):
    ok = f["penalty"] == 0
    at(t + 0.2 + i * 0.18, {(R, 14 + i): [("  " + NAMES.get(f["rule"], f["rule"]).ljust(21), "fg"),
                                         (f'{f["count"]:>5}', "fg" if ok else "red"),
                                         (f'{f["allowed"]:>7g}', "dim"),
                                         ("     ok" if ok else f'{-f["penalty"]:>6g}', "green" if ok else "red")]})
t += 0.2 + len(finds) * 0.18
nums = d1["facts"].get("added_numbers", [])
if nums:
    label = f'invented "{nums[0]}"'
    at(t, {(R, 14 + len(finds)): [("  " + label.ljust(21), "fg"), ("    1", "red"), ("      0", "dim"),
                                  (f'{-d1["fact_penalty"]:>6g}', "red")]})
for i, g in enumerate(geom):
    at(t + 0.4 + i * 0.12, {(L, 10 + i): feat_row(g, 1)})
t += 0.4 + len(geom) * 0.12 + 0.3
at(t, {(R, 22): [(f'  voice {d1["resemblance"]:g} − tells {d1["tell_penalty"]:g} − facts {d1["fact_penalty"]:g}', "dim")]})
at(t + 0.3, {(R, 23): [("  = ", "fg"), (f'{d1["overall"]:g}', "red"), ("  FAIL", "red"), (f'  below {cal["target"]:g}', "dim")],
             (L, 23): axis_row(cal["samples"], 1), (L, 25): [("  ●", "red"), (f' draft {d1["overall"]:g}', "dim")]})

# phase 3: revise
REV_T = 21.0
wipe = {(R, r): None for r in range(1, ROWS)}
wipe[(R, 0)] = [("  # pass 2: the agent revised from the notes", "dim")]
at(REV_T, wipe)
t = typed(REV_T + 0.6, R, 1, "ghostwriter score draft.md -s brief.md", per=0.035)
at(t + 0.2, {(R, 3 + i): [("  " + line, "fg")] for i, line in enumerate(PASS2)})
t += 0.9
at(t, {(R, 13): [("  patterns  ", "dim"), ("none above your own rate", "green")],
       (R, 14): [("  facts     ", "dim"), ("nothing added or dropped", "green")]})
for i, g in enumerate(geom):
    at(t + 0.3 + i * 0.12, {(L, 10 + i): feat_row(g, 2)})
t += 0.3 + len(geom) * 0.12 + 0.3
at(t, {(R, 22): [(f'  voice {d2["resemblance"]:g} − tells {d2["tell_penalty"]:g} − facts {d2["fact_penalty"]:g}', "dim")]})
at(t + 0.3, {(R, 23): [("  = ", "fg"), (f'{d2["overall"]:g}', "green"), ("  PASS", "green"), (f'  above {cal["target"]:g}', "dim")],
             (L, 23): axis_row(cal["samples"], 2), (L, 25): [("  ●", "green"), (f' draft {d2["overall"]:g}', "dim")]})

PHASES = [(0, "analyze"), (CAL_T, "calibrate"), (SCAN_T, "score"), (REV_T, "revise")]

# ---------------------------------------------------------------- render
events.sort(key=lambda e: e[0])
screen, timeline = {}, {}
for t_ev, ch in events:
    for key, val in ch.items():
        if screen.get(key) != val:
            screen[key] = val
            timeline.setdefault(key, []).append((t_ev, val))

css, body = [], []
uid = 0


def pct(t):
    return f"{max(0.0, min(100.0, t / T * 100)):.3f}%"


def visible(start, end, at_rest):
    """Switch an element on for [start, end) of each loop, with no fade."""
    global uid
    uid += 1
    name = f"k{uid}"
    frames = [f"0%{{opacity:{1 if start <= 0 else 0}}}"]
    if start > 0:
        frames.append(f"{pct(start)}{{opacity:1}}")
    if end < T:
        frames.append(f"{pct(end)}{{opacity:0}}")
    css.append(f"@keyframes {name}{{{''.join(frames)}}}.{name}{{opacity:{1 if at_rest else 0};animation:{name} {T}s step-end infinite}}")
    return name


def text_el(x, y, sg, cls=None, extra=""):
    n = sum(len(t) for t, _ in sg)
    spans = "".join(f'<tspan fill="{C[c]}">{escape(t)}</tspan>' for t, c in sg if t)
    c = f' class="{cls}"' if cls else ""
    # textLength pins every row to the cell grid, whatever monospace font renders it
    return f'<text{c} x="{x:.1f}" y="{y}" textLength="{n * CW:.1f}" lengthAdjust="spacingAndGlyphs" xml:space="preserve"{extra}>{spans}</text>'


for key, changes in timeline.items():
    pane, row = key
    x = PAD + (0 if pane == L else RIGHT_X) * CW
    for i, (start, sg) in enumerate(changes):
        if sg is None:
            continue
        end = changes[i + 1][0] if i + 1 < len(changes) else T
        body.append(text_el(x, TOP + row * LH, sg, visible(start, end, at_rest=(i == len(changes) - 1))))

# tmux status bar: window list, the current window in reverse video
bar = [f'<rect x="0" y="{STATUS_Y - 14}" width="{W}" height="20" fill="{C["green"]}"/>']
bar.append(text_el(PAD, STATUS_Y, [("[gw]", "bar")]))
x = PAD + 5 * CW
for i, (start, label) in enumerate(PHASES):
    end = PHASES[i + 1][0] if i + 1 < len(PHASES) else T
    word = f"{i}:{label}"
    on = visible(start, end, at_rest=(i == len(PHASES) - 1))
    # plain label underneath, then the reverse-video current window on top
    bar.append(text_el(x, STATUS_Y, [(word, "bar")]))
    bar.append(f'<rect class="{on}" x="{x - CW / 2:.1f}" y="{STATUS_Y - 14}" width="{(len(word) + 2) * CW:.1f}" height="20" fill="{C["bar"]}"/>')
    bar.append(text_el(x, STATUS_Y, [(word + "*", "green")], on, ' font-weight="bold"'))
    x += (len(word) + 3) * CW
right = '"ghostwriter" profile: me'
bar.append(text_el(W - PAD - len(right) * CW, STATUS_Y, [(right, "bar")]))

if problems:
    print("warning: range/score mismatch:", problems, file=sys.stderr)

border_x = PAD + LEFT_W * CW + CW / 2
doc = f"""<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}" role="img" aria-label="A tmux session: ghostwriter analyze turns six samples into per-measurement ranges and a pass mark of {cal['target']:g}; ghostwriter score flags AI patterns in a draft and scores it {d1['overall']:g}; the revised draft scores {d2['overall']:g} and passes">
<title>ghostwriter in a terminal: analyze, calibrate, score, revise</title>
<style>
text{{font-family:ui-monospace,'JetBrains Mono','SF Mono',Menlo,Consolas,'DejaVu Sans Mono',monospace;font-size:{FS}px;}}
{''.join(css)}
@media (prefers-reduced-motion:reduce){{*{{animation:none!important}}}}
</style>
<rect width="{W}" height="{H}" rx="10" fill="{C['bg']}"/>
<path d="M0 10a10 10 0 0 1 10-10h{W - 20}a10 10 0 0 1 10 10v18H0z" fill="{C['bar']}"/>
<circle cx="18" cy="14" r="5.5" fill="#ff5f57"/><circle cx="36" cy="14" r="5.5" fill="#febc2e"/><circle cx="54" cy="14" r="5.5" fill="#28c840"/>
<text x="{W / 2}" y="18" text-anchor="middle" fill="{C['dim']}" font-size="12">tmux</text>
<line x1="{border_x:.1f}" y1="{TOP - 14}" x2="{border_x:.1f}" y2="{STATUS_Y - 18}" stroke="{C['border']}"/>
{chr(10).join(body)}
{chr(10).join(bar)}
</svg>
"""
open(OUT, "w", encoding="utf-8").write(doc)
print(f"{len(doc)} bytes, {uid} timed elements")
