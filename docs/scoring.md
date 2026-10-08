# How ghostwriter scores a draft

```
overall = resemblance − AI-tell penalty − fact penalty
pass    = overall ≥ target  and  no blockers
```

A draft passes when it reads like the writer, shows no more AI patterns than
the writer's own writing does, and keeps the facts it was given.

## Why it works this way

A language model writes the most likely next word, so its default prose suits
the widest range of readers. A person writes for one reader in their own
uneven way. That gap shows up in two places, so ghostwriter measures both:

- **Missing voice**: average sentence rhythm, no quirks, none of the writer's
  phrases. Resemblance catches this.
- **Added machine habits**: "not X but Y" contrasts, one-line closers, "at its
  core", rule-of-three lists, dashes everywhere. The tell penalty catches this.

Measuring only resemblance lets a draft pass by sprinkling the writer's
markers on top of default AI prose. Measuring only tells removes the machine
but adds no person. And a rewrite
that sounds right but changes a number is worse than useless, hence the fact
check.

## Resemblance (0–100)

About 30 features compared with the writer's corpus:

| Group | Features |
| --- | --- |
| Rhythm | mean sentence length, its spread and variation, share of long (30+ words) and very short (≤6) sentences |
| Shape | sentences per paragraph, share of one-sentence paragraphs, word length, vocabulary richness |
| Flow | connectives, paragraphs opening with a connective, sentences opening with So/And/But/Also, the writer's usual openers |
| Punctuation | commas per sentence, long run-on sentences, semicolons, questions, exclamations, parentheses |
| Stance | first and second person, "I think"-style framing, hedges, intensifiers, contractions |
| Quirks | signature phrases, lowercase "i", stray capitals |
| Function words | cosine similarity of the small-word mix (the, and, so, just …) |

Each feature scores by how many standard deviations the draft sits from the
writer's mean, weighted by how much it says about identity. Details that keep
it fair:

- **Voice-density features** (connectives, hedges, first person, signature
  phrases …) get full credit from the writer's mean up to two standard
  deviations above it, and lose points beyond that. Too little sounds generic;
  stuffing them in sounds like a parody.
- **Spread floors** stop a writer who never used a semicolon from failing a
  draft over one.
- **Short drafts** are scored more gently, since rates over 100 words are noisy.
- **`--genre`** blends that genre's means with the overall voice, weighted by
  how many samples the genre has.

## AI-tell penalty

28 rule families, listed with fixes in `ghostwriter docs tells`.

| Strength | Cost per excess sighting (cap per rule) |
| --- | --- |
| hard | blocks the pass |
| strong | 6 (18) |
| medium | 3 (12) |
| weak | 1.5 (6), and only when another rule is also over |

**Excess** is measured against the writer: a rule their samples use is
allowed at 1.25 × their rate for the draft's length, plus one; a rule they
never use is allowed zero. Hard tells ignore the baseline. The total penalty
is capped at 60. Code, URLs, quoted speech and blockquotes are not checked.

## Fact penalty (with `--source`)

The draft is compared with the brief:

| Finding | Points | Blocks? |
| --- | --- | --- |
| number not in the brief | 5 each | yes |
| quotation not in the brief | 5 each | yes |
| capitalised name not in the brief | 3 each | no, but check it |
| number from the brief missing | 2 each | no |
| name from the brief missing | 1 each | no |

Capped at 30. Names are matched loosely ("Smith" after "Anna Smith" is fine),
so a flagged name is usually worth a look rather than certainly invented.

## Target and calibration

`analyze` (with 4+ samples) scores each sample against a profile built from
the others, and scores four built-in generic AI drafts against the full
profile. The target is the 25th percentile of the writer's own scores, so
three in four of their real pieces pass, kept within 55–90. If that would sit
within 5 points of the best AI draft, the profile is marked as not separating
the two and the target is set halfway between. Without calibration the
target is 75. `--target` overrides it.

Band: `close` at 80+, `medium` at 60+, else `far`.

## Reading `score --json`

`ghostwriter score draft.md -g email -s brief.md --json`, trimmed. This draft
reads like the writer but mentions "20 minutes", which is not in the brief:

```json
{
  "overall": 78.8, "resemblance": 83.8, "tell_penalty": 0, "fact_penalty": 5,
  "band": "medium", "target": 70, "pass": false,
  "blockers": ["adds numbers or quotations that are not in the source"],
  "words": 121, "genre": "email",
  "notes": [
    { "points": 5, "kind": "fact", "text": "Invented numbers not in the source; remove them or ask for the real figures: \"20\"" },
    { "points": 4, "kind": "voice", "text": "Hedging overdone (41 vs ~19 per 1000 words); keep only real doubt." }
  ],
  "tells": {
    "penalty": 0, "hard_fail": false,
    "findings": [
      { "rule": "signposting", "name": "filler signposting", "strength": "medium",
        "count": 1, "allowed": 1.8, "excess": 0, "penalty": 0, "fix": "…",
        "examples": [ { "rule": "signposting", "line": 1, "match": "at the end of the day" } ] }
    ]
  },
  "facts": { "added_numbers": ["20"] },
  "features": {
    "hedge_rate": { "draft": 41.32, "mean": 19.14, "z": 7.73, "score": 0.019, "points_lost": 4 }
  }
}
```

Here the writer says "at the end of the day" often enough that the signposting
rule allows it (`allowed` 1.8, `penalty` 0).

| Field | Meaning |
| --- | --- |
| `pass` | `overall ≥ target` and `blockers` is empty. |
| `blockers` | Reasons the draft fails whatever its score: a hard tell, or an invented number or quote. Omitted when empty. |
| `notes` | What to fix, sorted by `points` recovered. `kind` is `voice`, `tell` or `fact`. Tell notes quote each line as `L<line> "<match>"`. |
| `tells.findings` | Per rule: `count`, `allowed` (from the writer's rate), `excess`, `penalty`, `fix`, up to three `examples` with line numbers. |
| `facts` | Present only with `--source`. Empty lists are omitted. |
| `features` | Per feature: the draft's value, the writer's mean, `z` in standard deviations, `score` 0–1, `points_lost`. |

## Revising from the notes

Fix the top notes first; they recover the most points. Rewrite the paragraph
around a flagged phrase rather than swapping one word: patching phrase by
phrase tends to produce new tells. After each pass, reread for the patterns
that most often survive a rewrite: "not X but Y" contrasts, one-line closers,
rule-of-three lists, dashes and bold labels. Never fix a fact note by
inventing a source; remove the claim or ask for it.

## Limits

- Phrase lists (hedges, connectives, tells) are English. Other languages still
  get rhythm, shape, punctuation and signature-phrase features.
- Detection is evidence, not proof. People and detectors both misjudge AI text,
  which is why weak tells only count in company and why every rule respects the
  writer's own habits.
