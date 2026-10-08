# ghostwriter command reference

ghostwriter learns how someone writes from their own samples, then scores
drafts against that voice so an agent can write as them. It does not write
anything itself: the agent drafts, ghostwriter measures, the agent revises.

This is the complete manual. `ghostwriter docs cli` prints it, `ghostwriter
docs scoring` explains the score, `ghostwriter docs tells` lists every AI-tell
rule. `ghostwriter <command> --help` shows one command's flags.

## Typical workflow

```sh
ghostwriter add ~/writing/          # 1. collect samples (8+ across the genres you want)
ghostwriter analyze                 # 2. build the fingerprint and calibrate the pass mark
ghostwriter prompt style            # 3. have an agent write STYLE.md from this prompt
ghostwriter install                 # 4. install the drafting skill into this project
# 5. ask the agent to "write … as me"; it runs context → draft → score → revise
```

After adding or removing samples, run `analyze` again. Nothing else needs
rebuilding: the installed skill reads the profile through the CLI every time,
so it always sees the current voice.

## Global flags and environment

| Flag / variable | Meaning |
| --- | --- |
| `-p NAME`, `--profile NAME` | Which voice to use. Default `me`. Every command accepts it. |
| `GHOSTWRITER_PROFILE` | Default for `--profile`. |
| `GHOSTWRITER_HOME` | State directory. Default `~/.ghostwriter`. |
| `--version` | Print the version (`dev` for local builds). |
| `-h`, `--help` | Help for ghostwriter or for one command. |

Profile names use lowercase letters, digits, `.`, `_` and `-`, starting with a
letter or digit. A profile is created by the first `add` into it.

### Profiles

A profile is one independent voice: its own samples, fingerprint, AI-tell
baseline, pass mark and STYLE.md. Nothing is shared between profiles.

- **Profiles or genres?** Genres (`-g`) separate formats within one voice,
  such as your emails and your essays; they share the profile's baseline and
  pass mark. Use separate profiles for voices that should never blend: work
  and personal, different languages, or another writer (with their consent).
- **Managing them**: `profiles` lists them. To delete, rename or copy one, use
  the folder under `profiles/` directly. An installed skill names its profile,
  so reinstall it after a rename.
- **Skills**: each installed skill is bound to one profile. Install each voice
  under its own `--name`, and set `--writer` so you, and the agent, can tell
  them apart.

```sh
ghostwriter -p work add ~/work-emails/
ghostwriter -p work analyze
ghostwriter -p work install --name ghostwriter-work -w "Jakub (work)"
export GHOSTWRITER_PROFILE=work       # default for this shell
```

### State on disk

```
~/.ghostwriter/
└── profiles/<name>/
    ├── corpus.jsonl    samples, verbatim, one JSON object per line
    ├── profile.json    the fingerprint written by analyze
    └── STYLE.md        the style guide an agent writes (see prompt style)
```

Everything is local. Samples never leave the machine.

### Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success. For `score`, the draft passed. |
| 1 | Runtime error (message on stderr), `score` failed the draft, `tells` found a hard tell, or `add` added nothing. |
| 80 | Usage error: unknown command, missing argument or bad flag. |

## add

Add writing samples to a profile.

```sh
ghostwriter add <path>... [-g GENRE] [--name NAME]
```

| Flag | Meaning |
| --- | --- |
| `-g`, `--genre` | Genre for every sample added by this call. Default: guessed from each filename. |
| `--name` | Sample name when reading stdin (`-`). Default `stdin.txt`. |

Each `path` is a file, a directory, or `-` for stdin.

**Directories are walked recursively**, every subdirectory included, except:

- hidden directories (`.git/`, `.obsidian/` …) are skipped with everything
  inside them; the directory you name is always walked even if it is hidden;
- hidden files (`.DS_Store` …) are skipped;
- files with unsupported extensions are ignored silently;
- symlinked directories are not followed.

**Supported files**

| Extension | How it is read |
| --- | --- |
| `.txt` `.md` `.markdown` `.text` | As is. |
| `.docx` | Paragraph text from the Word document. |
| `.eml` | The body only: headers, the quoted thread after "On … wrote:" and `>` lines are dropped, since they are not the writer's words. |
| `.pdf` | Through poppler's `pdftotext`, which must be on `PATH` (`brew install poppler`). |

Text is kept **verbatim**: typos, odd capitals and run-ons are part of the
fingerprint. Only line endings and runs of blank lines are normalised.

**Genre** comes from `--genre` if given, otherwise from a keyword in the
*filename* (not the folder name), checked in this order:

| Filename contains | Genre |
| --- | --- |
| `email`, `mail`, `letter`, `message` (or the file is `.eml`) | `email` |
| `essay` | `essay` |
| `report`, `memo` | `report` |
| `article`, `blog`, `post`, `newsletter` | `article` |
| `answer`, `reply`, `comment`, `review` | `answer` |
| `slack`, `chat` | `chat` |
| `tweet`, `linkedin` | `social` |
| anything else | `other` |

Writing sorted into folders keeps its genre if each folder is added
separately: `ghostwriter add ~/writing/emails/ -g email`.

**Duplicates**: a sample with the same name or the same text as an existing
one replaces it (`updated` in the output), so re-adding a folder after editing
files is safe.

Samples under 20 words, and files that fail to read, are reported on stderr
and skipped; the rest are still added. If nothing could be added, `add` exits
1. A path that does not exist stops the whole call before anything is added.

```sh
ghostwriter add ~/Documents/writing/
ghostwriter add notes/q3-update.md -g report
ghostwriter add ~/Mail/sent/*.eml
pbpaste | ghostwriter add - --name slack_rant.txt -g chat
ghostwriter -p work add ~/work-emails/
```

**What makes good samples**: your own unedited writing, 8 or more pieces of a
few hundred words each, covering the genres you want drafted. Leave out text
someone else wrote or heavily edited (co-authored docs, quoted replies,
pasted AI output): it blurs the fingerprint.

## ls

List the samples in a profile.

```sh
ghostwriter ls [--json]
```

| Flag | Meaning |
| --- | --- |
| `--json` | Print `[{"name", "genre", "words"}]`. |

`list` is an alias.

## rm

Remove samples by the name `ls` shows.

```sh
ghostwriter rm <name>...
```

All or nothing: if any name is not in the profile, nothing is removed. Run
`analyze` afterwards to update the fingerprint.

## profiles

List every profile with its sample count, whether it has been analyzed, its
pass mark and whether STYLE.md exists. `*` marks the active profile (from
`-p` or `GHOSTWRITER_PROFILE`).

```sh
ghostwriter profiles
```

## analyze

Build the fingerprint (`profile.json`) from the samples and calibrate the pass
mark.

```sh
ghostwriter analyze [--exclude SUBSTR] [--json]
```

| Flag | Meaning |
| --- | --- |
| `--exclude` | Leave out samples whose name contains this text (case-insensitive). For checking how a held-out piece scores. |
| `--json` | Print the whole profile as JSON instead of the summary. |

What it records:

- ~30 stylometric features per sample, with mean, spread, min and max across
  the corpus, and per-genre means;
- signature phrases: 2–4 word phrases that recur across samples;
- usual sentence openers;
- the writer's own rate of every AI-tell pattern, so scoring can allow what
  they really do;
- calibration (with 4+ samples): each sample scored against a profile built
  from the others, plus built-in generic AI drafts; the pass mark is set
  between them. With fewer samples the pass mark is 75.

The summary prints this fingerprint in plain words and the calibration table.
"Your samples and generic AI drafts overlap" means the profile cannot yet tell
your writing from a default AI draft: add more, and more varied, samples.

`profile.json` is tied to the ghostwriter version that built it; if a newer
version changes the features, commands ask you to run `analyze` again.

```sh
ghostwriter analyze
ghostwriter analyze --exclude running && ghostwriter score essays/running.md -g essay
ghostwriter analyze     # rebuild with everything afterwards
```

## score

Score a draft against the voice. This is the command an agent runs in its
revise loop. `ghostwriter docs scoring` explains every number in the result.

```sh
ghostwriter score <draft|-> [-g GENRE] [-t TARGET] [-s SOURCE] [--json]
```

| Flag | Meaning |
| --- | --- |
| `-g`, `--genre` | Blend in that genre's style. If the profile has no such genre, a note goes to stderr and the overall voice is used. |
| `-t`, `--target` | Pass mark. Default: the calibrated one (75 if uncalibrated). |
| `-s`, `--source` | The brief or original text. Flags numbers, names and quotes the draft adds or drops; an invented number or quote blocks the pass. |
| `--json` | Machine-readable result. |

The draft can be any supported file type, or `-` for stdin. Exits 0 on pass,
1 on fail, so it works in scripts.

```sh
ghostwriter score draft.md
ghostwriter score draft.md -g email -s brief.md --json
pbpaste | ghostwriter score -
ghostwriter score post.md -g article && echo "ship it"
```

## tells

List AI-writing patterns in one or more texts, with line numbers. Works
without a profile.

```sh
ghostwriter tells <file|->... [--absolute] [--json]
```

| Flag | Meaning |
| --- | --- |
| `--absolute` | Ignore the writer's baseline and flag every sighting. Automatic when the profile does not exist or is not analyzed. |
| `--json` | A list with one entry per file: `file`, `penalty`, `hard_fail`, `findings`, `hits`. |

With an analyzed profile, a pattern only costs points beyond the writer's own
rate (see `ghostwriter docs tells`). Exits 1 if any file has a hard tell
(chatbot citation markup, an unfilled placeholder like `[Your Name]`, "as an
AI"), so it can gate commits or CI.

```sh
ghostwriter tells article.md
ghostwriter tells docs/*.md --absolute
pbpaste | ghostwriter tells -
```

## context

Print what an agent reads before drafting: STYLE.md, the fingerprint in plain
words (including the AI patterns this writer never uses), and one real excerpt
per genre to mirror. The excerpt is the sample most typical of that genre,
trimmed to about 2400 characters at a paragraph break.

```sh
ghostwriter context [-g GENRE]
```

| Flag | Meaning |
| --- | --- |
| `-g`, `--genre` | Show only that genre's excerpt (all genres if the profile has none of it). |

If STYLE.md does not exist yet, the output starts with instructions to write it
first.

## prompt style

Print the prompt that has an agent write STYLE.md: what to cover, the
fingerprint, every sample (each trimmed to about 6000 characters), and the
exact path to save it to.

```sh
ghostwriter prompt style
ghostwriter prompt style | pbcopy
```

STYLE.md is free text and yours to edit. The scorer does not read it; it guides
the agent. Re-run the prompt after adding many new samples.

## install

Install the drafting skill, which follows the
[Agent Skills specification](https://agentskills.io/specification).

```sh
ghostwriter install [--dir DIR] [--name NAME] [-w WRITER] [--max-passes N]
```

| Flag | Meaning |
| --- | --- |
| `--dir` | Skills directory. Default `.agents/skills` under the current directory. `~` is expanded. |
| `--name` | Skill name and folder. Default `ghostwriter`. Lowercase letters, digits and single hyphens, at most 64 characters. |
| `-w`, `--writer` | How the skill refers to the writer. Default: the profile name, or "the user" for the `me` profile. |
| `--max-passes` | Score-and-revise rounds before the agent stops. Default 4. |

It writes:

```
<dir>/<name>/
├── SKILL.md              the write → score → revise loop, bound to this profile
└── references/
    ├── cli.md            this manual
    ├── scoring.md        how to read a score
    └── tells.md          every AI-tell rule and its fix
```

Reinstalling overwrites a skill ghostwriter wrote, and refuses to touch a
skill folder of the same name that it did not write. Where agents look for
skills: Codex reads `.agents/skills`; Claude Code reads `.claude/skills`; use
`--dir ~/.agents/skills` (or the harness's global folder) for every project.
`install` warns if `ghostwriter` is not on `PATH`, since the agent has to run it.

One skill serves one profile. For several voices, install each under its own
name:

```sh
ghostwriter install
ghostwriter install --dir .claude/skills
ghostwriter -p work install --name ghostwriter-work -w "Jakub (work)"
```

## docs

Print this manual or one of its companions.

```sh
ghostwriter docs [cli|scoring|tells]
```

`cli` (default) is this reference, `scoring` explains the score and its JSON,
`tells` lists every AI-tell rule.

## Setting it up for someone through an agent

An agent asked to "set up ghostwriter for me" should:

1. Run `ghostwriter docs` (this file).
2. Ask where the user's own writing is, then `ghostwriter add` it, using `-g`
   per folder if filenames carry no genre.
3. Run `ghostwriter analyze` and report the calibration line.
4. Run `ghostwriter prompt style`, write STYLE.md to the path it names, and
   ask the user to read it.
5. Run `ghostwriter install` (with `--dir` for the user's harness).
