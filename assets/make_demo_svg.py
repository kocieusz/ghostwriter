"""Generate assets/demo.svg: the README hero, a tmux session using ghostwriter.

    python3 assets/make_demo_svg.py profile.json draft1.json draft2.json assets/demo.svg

Inputs are real ghostwriter output against a profile built from
internal/voice/testdata/writer:

    ghostwriter analyze --json                                         > profile.json
    ghostwriter score pass1.md -g email --source brief.md --json       > draft1.json
    ghostwriter score pass2.md -g email --source brief.md --json       > draft2.json

where pass1.md and pass2.md hold PASS1 and PASS2 below, and brief.md is BRIEF.
Scores, word counts, flagged lines and the pass mark come from those files;
the summary lines use the CLI's own wording. Drawing is done by termsvg.py,
shared with make_engine_svg.py.
"""
import json
import sys

from termsvg import Session

prof, d1, d2 = (json.load(open(p)) for p in sys.argv[1:4])
OUT = sys.argv[4]

BRIEF = "Reply to Marta: yes to Thursday's demo. Slides by Wednesday. Payment API sandbox still down."
PASS1 = """Hi Marta,

Thank you for reaching out! I'm thrilled to confirm Thursday's demo —
it's not just a meeting, it's a pivotal moment for the project.

I will ensure the slides are ready by Wednesday, highlighting our robust
progress. The payment API sandbox is down, and 47 tickets are blocked.

Let me know if you'd like me to prepare anything else!""".split("\n")
# Phrases the scorer penalised. The em dash is not here: this writer uses dashes,
# so the scorer allowed it.
FLAGS = ["it's not just a meeting, it's", "pivotal moment", ", highlighting", "robust", "47",
         "Let me know if you'd like me"]
PASS2 = """Hey Marta,

Thursday works, I'll be there. Slides will be ready by Wednesday, nothing
fancy, just the numbers and a couple of screenshots.

One thing though, the payment API sandbox is still down, so honestly I
can't show that part live. I'd rather tell them now than find out during
the demo haha.

Thanks a lot!""".split("\n")

s = Session(0, rows=24)   # loop length is set once the script's length is known
cal = prof["calibration"]
own = [x["overall"] for x in cal["samples"] if not x.get("ai")]
ai = [x["overall"] for x in cal["samples"] if x.get("ai")]
SCORE_CMD = "ghostwriter score draft.md -g email --source brief.md"


def row(r, *parts):
    return {(0, r): list(parts)}


def summary(t, r, res):
    """The first lines `ghostwriter score` prints, in its own wording."""
    ok = res["pass"]
    s.at(t, row(r, ("Voice match: ", "fg"), (f'{res["overall"]:g}', "green" if ok else "red"), ("/100", "dim"),
                (f"  ({res['band']}, target {res['target']:g}) — ", "dim"),
                ("PASS" if ok else "needs work", "green" if ok else "red")))
    s.at(t + 0.2, row(r + 1, (f'  resemblance {res["resemblance"]:.1f}  − AI tells {res["tell_penalty"]:.1f}'
                              f'  − facts {res["fact_penalty"]:.1f}   ({res["words"]} words)', "dim")))


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


# ---------------------------------------------------------------- 0: setup
t = s.typed(0.5, (0, 0), "ghostwriter add ~/writing/")
for i, d in enumerate(prof["docs"]):
    s.at(t + 0.2 + i * 0.12, row(1 + i, (f'  added    {d["name"]:<40} {d["genre"]:<8} {d["words"]:>5}w', "dim")))
t += 0.2 + len(prof["docs"]) * 0.12 + 0.2
s.at(t, row(8, (f'{len(prof["docs"])} samples in profile "me".', "fg")))
t = s.typed(t + 0.6, (0, 10), "ghostwriter analyze")
s.at(t + 0.4, row(11, ("  your samples ", "dim"), (f"~{sum(own) / len(own):.0f}", "green"),
                  (f" (p25 {cal['loo_p25']:.0f}), generic AI drafts ", "dim"), (f"~{sum(ai) / len(ai):.0f}", "red"),
                  (f" (best {max(ai):.0f}) → pass mark ", "dim"), (f"{cal['target']:g}", "yellow")))
t = s.typed(t + 1.2, (0, 13), "ghostwriter install")
s.at(t + 0.3, row(14, ('Installed skill "ghostwriter" (profile "me") in ', "fg"), (".agents/skills/ghostwriter", "blue")))

# ---------------------------------------------------------------- 1: draft
DRAFT_T = t + 1.6          # each scene starts after the last one has finished
s.clear(DRAFT_T)
s.at(DRAFT_T + 0.1, row(0, ("you › ", "magenta"), ("write a reply to Marta as me", "fg")))
s.at(DRAFT_T + 0.7, row(1, ("  # the agent drafts draft.md, then checks it", "dim")))
s.at(DRAFT_T + 1.3, {(0, 3 + i): [("  " + line, "fg")] for i, line in enumerate(PASS1) if line})
t = s.typed(DRAFT_T + 2.0, (0, 13), SCORE_CMD, per=0.035)
summary(t + 0.4, 15, d1)
if d1.get("blockers"):
    s.at(t + 0.8, row(17, ("  ✗ " + d1["blockers"][0], "red")))
for i, line in enumerate(PASS1):     # the flagged phrases light up, line by line
    if line:
        s.at(t + 1.2 + i * 0.1, {(0, 3 + i): flagged(line)})
t += 2.4
s.at(t, row(19, ("What to change (most points first):", "fg")))
finds = {f["rule"]: f for f in d1["tells"]["findings"]}
notes = []
for rule, label in (("negative_parallelism", '"not X but Y" contrast'), ("chatbot_residue", "assistant wrapper")):
    f = finds[rule]
    ex = f["examples"][0]
    notes.append([("  ✗ ", "red"), (f'[tell −{f["penalty"]:.1f}] ', "dim"), (f"{label:<24}", "yellow"),
                  (f'L{ex["line"]} "{ex["match"]}"', "fg")])
added = d1.get("facts", {}).get("added_numbers", [])
if added:
    notes.append([("  ✗ ", "red"), (f'[fact −{5 * len(added):.1f}] ', "dim"), (f"{'invented number':<24}", "yellow"),
                  (f'"{added[0]}" is not in the brief', "fg")])
for i, n in enumerate(notes):
    s.at(t + 0.3 + i * 0.25, row(20 + i, *n))
t += 0.3 + len(notes) * 0.25

# ---------------------------------------------------------------- 2: revise
REV_T = t + 3.2            # time to read the notes
s.clear(REV_T)
s.at(REV_T + 0.1, row(0, ("  # pass 2: the agent rewrote the draft from the notes", "dim")))
s.at(REV_T + 0.6, {(0, 2 + i): [("  " + line, "fg")] for i, line in enumerate(PASS2) if line})
t = s.typed(REV_T + 1.6, (0, 13), SCORE_CMD, per=0.035)
summary(t + 0.4, 15, d2)
s.at(t + 1.4, row(18, ("→ ", "green"), ("delivered in your voice, nothing invented", "fg")))
s.loop = t + 5.0           # hold the passing result before looping

size, n = s.render(
    OUT, windows=[(0, "setup"), (DRAFT_T, "draft"), (REV_T, "revise")], status_right='"ghostwriter" profile: me',
    title="ghostwriter: learn your voice, score the draft, revise until it passes",
    aria=(f"A tmux session: ghostwriter learns a voice from six samples, scores an AI-sounding draft "
          f"{d1['overall']:g}/100 for AI patterns and an invented number, then passes the revised draft at "
          f"{d2['overall']:g}/100"))
print(f"{size} bytes, {n} timed elements")
