// ghostwriter learns a writer's voice from their own samples and scores drafts
// against it, so a coding agent can write as them: same rhythm and quirks, no
// AI tells, no invented facts.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/kocieusz/ghostwriter/internal/ingest"
	"github.com/kocieusz/ghostwriter/internal/skill"
	"github.com/kocieusz/ghostwriter/internal/store"
	"github.com/kocieusz/ghostwriter/internal/stylo"
	"github.com/kocieusz/ghostwriter/internal/tells"
	"github.com/kocieusz/ghostwriter/internal/voice"
)

type cli struct {
	Profile string           `short:"p" help:"Voice profile to use." env:"GHOSTWRITER_PROFILE" default:"me" placeholder:"NAME"`
	Version kong.VersionFlag `help:"Print version and exit."`

	Add      addCmd      `cmd:"" help:"Add writing samples (.txt .md .docx .pdf .eml, folders, or - for stdin)."`
	Ls       lsCmd       `cmd:"" aliases:"list" help:"List the samples in a profile."`
	Rm       rmCmd       `cmd:"" help:"Remove samples from a profile."`
	Profiles profilesCmd `cmd:"" help:"List profiles."`
	Analyze  analyzeCmd  `cmd:"" help:"Build the fingerprint from the samples and calibrate the pass mark."`
	Score    scoreCmd    `cmd:"" help:"Score a draft against the voice (exit 1 if it fails)."`
	Tells    tellsCmd    `cmd:"" help:"List AI-writing tells in one or more texts (exit 1 on a hard tell)."`
	Context  contextCmd  `cmd:"" help:"Print what an agent needs before drafting: STYLE.md, fingerprint, excerpts."`
	Prompt   promptCmd   `cmd:"" help:"Print the prompt that has your agent write STYLE.md."`
	Install  installCmd  `cmd:"" help:"Install the drafting skill into ./.agents/skills (or --dir)."`
	Docs     docsCmd     `cmd:"" help:"Print the manual: cli (default), scoring, or tells."`
}

func main() {
	var c cli
	ctx := kong.Parse(&c,
		kong.Name("ghostwriter"),
		kong.Description("Draft in your own voice: learn it from your writing, score drafts against it, catch AI tells.\nRun `ghostwriter docs` for the full manual."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
		kong.Vars{"version": version()},
	)
	err := ctx.Run(&c)
	if errors.Is(err, errFailed) {
		os.Exit(1)
	}
	ctx.FatalIfErrorf(err)
}

// errFailed exits 1 without a message: the command already said why.
var errFailed = errors.New("failed")

// buildVersion is stamped in by goreleaser (-X main.buildVersion=…); local
// builds report "dev".
var buildVersion string

func version() string {
	if buildVersion != "" {
		return buildVersion
	}
	return "dev"
}

func (c *cli) open() (store.Profile, error) { return store.Open(c.Profile) }

// load opens the profile and its fingerprint.
func (c *cli) load() (store.Profile, *voice.Profile, error) {
	sp, err := c.open()
	if err != nil {
		return sp, nil, err
	}
	if !sp.Exists() {
		return sp, nil, fmt.Errorf("no profile %q; start one with `ghostwriter add -p %s <your writing>`", sp.Name, sp.Name)
	}
	vp, err := sp.Load()
	return sp, vp, err
}

// ---------------------------------------------------------------- add

type addCmd struct {
	Paths []string `arg:"" help:"Files or folders of your writing; - reads stdin."`
	Genre string   `short:"g" help:"Genre for every added sample (default: from the filename: email, essay, report, article, answer…)."`
	Name  string   `help:"Sample name when reading stdin." default:"stdin.txt"`
}

func (a *addCmd) Run(c *cli) error {
	sp, err := c.open()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	var files []string
	for _, p := range a.Paths {
		if p == "-" {
			files = append(files, "-")
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			files = append(files, p)
			continue
		}
		err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && path != p && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if !d.IsDir() && ingest.Supported(path) && !strings.HasPrefix(d.Name(), ".") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("no samples found (supported: %s)", strings.Join(ingest.Extensions, " "))
	}

	added := 0
	for _, f := range files {
		text, err := ingest.Read(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ! skipped %s: %v\n", f, err)
			continue
		}
		name := filepath.Base(f)
		if f == "-" {
			name = a.Name
		}
		words := stylo.WordCount(text)
		if words < 20 {
			fmt.Fprintf(os.Stderr, "  ! skipped %s: only %d words\n", name, words)
			continue
		}
		genre := a.Genre
		if genre == "" {
			genre = ingest.Genre(name)
		}
		d := voice.Doc{Name: name, Genre: genre, Words: words, Hash: store.Hash(text), Text: text}
		replaced := false
		for i := range docs {
			if docs[i].Name == d.Name || docs[i].Hash == d.Hash {
				docs[i], replaced = d, true
				break
			}
		}
		if !replaced {
			docs = append(docs, d)
		}
		verb := "added"
		if replaced {
			verb = "updated"
		}
		fmt.Printf("  %-8s %-40s %-8s %5dw\n", verb, name, genre, words)
		added++
	}
	if added == 0 {
		return errFailed
	}
	if err := sp.SaveCorpus(docs); err != nil {
		return err
	}
	fmt.Printf("\n%s in profile %q. Next: ghostwriter analyze -p %s\n", plural(len(docs), "sample"), sp.Name, sp.Name)
	if len(docs) < 8 {
		fmt.Println("Tip: 8+ samples across the genres you want drafted give a much sharper fingerprint.")
	}
	return nil
}

// ---------------------------------------------------------------- ls / rm / profiles

type lsCmd struct {
	JSON bool `help:"Emit JSON."`
}

func (l *lsCmd) Run(c *cli) error {
	sp, err := c.open()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	if l.JSON {
		type row struct {
			Name  string `json:"name"`
			Genre string `json:"genre"`
			Words int    `json:"words"`
		}
		rows := []row{}
		for _, d := range docs {
			rows = append(rows, row{d.Name, d.Genre, d.Words})
		}
		return printJSON(rows)
	}
	if len(docs) == 0 {
		fmt.Printf("Profile %q has no samples. Add some with `ghostwriter add -p %s <files>`.\n", sp.Name, sp.Name)
		return nil
	}
	total := 0
	for _, d := range docs {
		fmt.Printf("  %-40s %-8s %5dw\n", d.Name, d.Genre, d.Words)
		total += d.Words
	}
	fmt.Printf("\n%s, %s in %s\n", plural(len(docs), "sample"), plural(total, "word"), sp.Dir)
	return nil
}

type rmCmd struct {
	Names []string `arg:"" help:"Sample names as shown by ls."`
}

func (r *rmCmd) Run(c *cli) error {
	sp, err := c.open()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	var keep []voice.Doc
	removed := map[string]bool{}
	for _, d := range docs {
		hit := false
		for _, n := range r.Names {
			if d.Name == n {
				hit = true
			}
		}
		if hit {
			removed[d.Name] = true
		} else {
			keep = append(keep, d)
		}
	}
	for _, n := range r.Names {
		if !removed[n] {
			return fmt.Errorf("no sample %q in profile %q", n, sp.Name)
		}
	}
	if err := sp.SaveCorpus(keep); err != nil {
		return err
	}
	fmt.Printf("Removed %s. Re-run `ghostwriter analyze -p %s` to update the fingerprint.\n", plural(len(removed), "sample"), sp.Name)
	return nil
}

type profilesCmd struct{}

func (profilesCmd) Run(c *cli) error {
	names, err := store.List()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		fmt.Println("No profiles yet. Start one with `ghostwriter add <your writing>`.")
		return nil
	}
	for _, n := range names {
		mark := " "
		if n == c.Profile {
			mark = "*"
		}
		sp, _ := store.Open(n)
		docs, _ := sp.Corpus()
		status := "not analyzed"
		if vp, err := sp.Load(); err == nil {
			status = "analyzed"
			if vp.Calibration != nil {
				status = fmt.Sprintf("analyzed, pass mark %.0f", vp.Calibration.Target)
			}
		}
		style := ""
		if s, _ := sp.Style(); s == "" {
			style = ", no STYLE.md"
		}
		fmt.Printf("%s %-20s %11s  (%s%s)\n", mark, n, plural(len(docs), "sample"), status, style)
	}
	return nil
}

// ---------------------------------------------------------------- analyze

type analyzeCmd struct {
	Exclude string `help:"Leave out samples whose name contains this (manual leave-one-out)." placeholder:"SUBSTR"`
	JSON    bool   `help:"Print the profile as JSON."`
}

func (a *analyzeCmd) Run(c *cli) error {
	sp, err := c.open()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	if a.Exclude != "" {
		var keep []voice.Doc
		for _, d := range docs {
			if !strings.Contains(strings.ToLower(d.Name), strings.ToLower(a.Exclude)) {
				keep = append(keep, d)
			}
		}
		docs = keep
	}
	if len(docs) == 0 {
		return fmt.Errorf("profile %q has no samples; add some with `ghostwriter add -p %s <files>`", sp.Name, sp.Name)
	}
	vp := voice.Build(docs)
	cal := voice.Calibrate(vp, docs)
	if err := sp.Save(vp); err != nil {
		return err
	}
	if a.JSON {
		return printJSON(vp)
	}

	fmt.Printf("Fingerprint written to %s\n\n", sp.ProfilePath())
	fmt.Print(skill.Fingerprint(vp))
	fmt.Println()
	if cal == nil {
		fmt.Printf("Calibration skipped: needs %d+ samples (have %d). Scoring uses a default pass mark of %d.\n",
			voice.MinCalibrationDocs, len(docs), voice.DefaultTarget)
	} else {
		fmt.Println("Calibration (each sample scored against a profile built from the others):")
		for _, s := range cal.Samples {
			label := s.Name
			if s.AI {
				label = "generic AI " + strings.TrimPrefix(s.Name, "ai:")
			}
			fmt.Printf("  %-40s %5.1f\n", label, s.Overall)
		}
		fmt.Printf("\n  your samples ~%.0f (p25 %.0f), generic AI drafts ~%.0f (best %.0f) → pass mark %.0f\n",
			cal.LOOMean, cal.LOOLow, cal.AIMean, cal.AIMax, cal.Target)
		if !cal.Separated {
			fmt.Println("  ! Your samples and generic AI drafts overlap. Add more (and more varied) samples to sharpen the fingerprint.")
		}
	}
	if s, _ := sp.Style(); s == "" {
		fmt.Printf("\nNext: have your agent write STYLE.md. Give it the output of\n  ghostwriter prompt style -p %s\n", sp.Name)
	}
	return nil
}

// ---------------------------------------------------------------- score

type scoreCmd struct {
	Draft  string  `arg:"" help:"Draft to score (.md .txt .docx …), or - for stdin."`
	Genre  string  `short:"g" help:"Score against this genre's style as well as the overall voice."`
	Target float64 `short:"t" help:"Pass mark (default: the calibrated one)."`
	Source string  `short:"s" help:"Brief or original text; flags numbers, names and quotes the draft adds or drops." type:"path" placeholder:"FILE"`
	JSON   bool    `help:"Emit JSON (for the agent loop)."`
}

func (s *scoreCmd) Run(c *cli) error {
	_, vp, err := c.load()
	if err != nil {
		return err
	}
	draft, err := ingest.Read(s.Draft)
	if err != nil {
		return err
	}
	opt := voice.Options{Genre: s.Genre, Target: s.Target}
	if s.Source != "" {
		if opt.Source, err = ingest.Read(s.Source); err != nil {
			return err
		}
	}
	if s.Genre != "" {
		if _, ok := vp.Genres[s.Genre]; !ok {
			fmt.Fprintf(os.Stderr, "note: profile has no %q samples; scoring against the overall voice (genres: %s)\n",
				s.Genre, strings.Join(skill.Genres(vp), ", "))
		}
	}
	if stylo.WordCount(draft) == 0 {
		name := s.Draft
		if name == "-" {
			name = "stdin"
		}
		return fmt.Errorf("the draft (%s) has no words to score", name)
	}
	r := voice.Score(vp, draft, opt)
	if s.JSON {
		if err := printJSON(r); err != nil {
			return err
		}
	} else {
		printScore(r)
	}
	if !r.Pass {
		return errFailed
	}
	return nil
}

func printScore(r voice.Result) {
	status := "PASS"
	if !r.Pass {
		status = "needs work"
	}
	fmt.Printf("\nVoice match: %.1f/100  (%s, target %.0f) — %s\n", r.Overall, r.Band, r.Target, status)
	fmt.Printf("  resemblance %.1f  − AI tells %.1f", r.Resemblance, r.TellPenalty)
	if r.Facts != nil {
		fmt.Printf("  − facts %.1f", r.FactPenalty)
	}
	fmt.Printf("   (%d words)\n", r.Words)
	for _, b := range r.Blockers {
		fmt.Printf("  ✗ %s\n", b)
	}
	fmt.Println()
	if len(r.Notes) == 0 {
		fmt.Println("No notable deviations: reads like the writer.")
		return
	}
	fmt.Println("What to change (most points first):")
	for i, n := range r.Notes {
		fmt.Printf("  %2d. [%s −%.1f] %s\n", i+1, n.Kind, n.Points, n.Text)
	}
	fmt.Println()
}

// ---------------------------------------------------------------- tells

type tellsCmd struct {
	Files    []string `arg:"" help:"Texts to check, or - for stdin."`
	Absolute bool     `help:"Ignore the writer's baseline; flag every sighting."`
	JSON     bool     `help:"Emit JSON: one entry per file."`
}

// tellsReport is one file's result in JSON output.
type tellsReport struct {
	File string `json:"file"`
	tells.Assessment
	Hits []tells.Hit `json:"hits"`
}

func (t *tellsCmd) Run(c *cli) error {
	var base map[string]tells.Baseline
	if !t.Absolute {
		if sp, err := c.open(); err == nil && sp.Exists() {
			if vp, err := sp.Load(); err == nil {
				base = vp.Tells
			}
		}
	}
	var reports []tellsReport
	for _, f := range t.Files {
		text, err := ingest.Read(f)
		if err != nil {
			return fmt.Errorf("%s: %w", f, err)
		}
		hits := tells.Detect(text)
		reports = append(reports, tellsReport{f, tells.Assess(hits, stylo.WordCount(text), base), hits})
	}
	hard := false
	for _, r := range reports {
		hard = hard || r.HardFail
	}
	if t.JSON {
		if err := printJSON(reports); err != nil {
			return err
		}
	} else {
		for i, r := range reports {
			if len(reports) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Printf("== %s\n", r.File)
			}
			printTells(r, base, c.Profile)
		}
	}
	if hard {
		return errFailed
	}
	return nil
}

func printTells(r tellsReport, base map[string]tells.Baseline, profile string) {
	if len(r.Hits) == 0 {
		fmt.Println("No AI tells found.")
		return
	}
	for _, h := range r.Hits {
		rule := tells.ByID(h.Rule)
		fmt.Printf("  L%-4d %-7s %-28s %q\n", h.Line, rule.Strength, h.Rule, h.Match)
	}
	fmt.Println()
	for _, f := range r.Findings {
		if f.Penalty == 0 {
			continue
		}
		fmt.Printf("  −%-5.1f %s ×%d: %s\n", f.Penalty, f.Name, f.Count, f.Fix)
	}
	fmt.Printf("\nTell penalty: %.1f", r.Penalty)
	if base == nil {
		fmt.Print(" (absolute; no writer baseline)")
	} else {
		fmt.Printf(" (relative to profile %q)", profile)
	}
	fmt.Println()
}

// ---------------------------------------------------------------- context / prompt

type contextCmd struct {
	Genre string `short:"g" help:"Show the excerpt for this genre only."`
}

func (x *contextCmd) Run(c *cli) error {
	sp, vp, err := c.load()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	style, err := sp.Style()
	if err != nil {
		return err
	}
	fmt.Print(skill.Context(sp.Name, style, vp, docs, x.Genre))
	return nil
}

type promptCmd struct {
	Style promptStyleCmd `cmd:"" help:"Prompt for authoring STYLE.md from your samples."`
}

type promptStyleCmd struct{}

func (promptStyleCmd) Run(c *cli) error {
	sp, vp, err := c.load()
	if err != nil {
		return err
	}
	docs, err := sp.Corpus()
	if err != nil {
		return err
	}
	out, err := skill.StylePrompt(sp.Name, sp.StylePath(), vp, docs)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return nil
}

// ---------------------------------------------------------------- install

type installCmd struct {
	Dir       string `help:"Skills directory to install into (default: .agents/skills in the current directory)." placeholder:"DIR"`
	Name      string `help:"Skill name and folder; lowercase letters, digits and hyphens." default:"ghostwriter"`
	Writer    string `short:"w" help:"How prompts refer to you (default: the profile name)."`
	MaxPasses int    `help:"Score-and-revise passes before the agent stops." default:"4"`
}

func (i *installCmd) Run(c *cli) error {
	sp, vp, err := c.load()
	if err != nil {
		return err
	}
	v := skill.SkillVars{
		SkillName: i.Name, Writer: i.Writer, Profile: sp.Name,
		Genres: strings.Join(skill.Genres(vp), ", "), MaxPasses: i.MaxPasses,
	}
	if v.Writer == "" {
		v.Writer = sp.Name
		if sp.Name == "me" {
			v.Writer = "the user"
		}
	}
	dir := skill.DefaultDir
	if i.Dir != "" {
		if dir, err = store.ExpandHome(i.Dir); err != nil {
			return err
		}
	}
	topics := docTopics()
	dst, err := skill.Install(dir, v, map[string]string{"cli.md": topics["cli"], "scoring.md": topics["scoring"]})
	if err != nil {
		return err
	}
	fmt.Printf("Installed skill %q (profile %q) in %s\n", v.SkillName, sp.Name, dst)
	fmt.Println("In a new agent session here, ask it to \"write … as me\".")
	if s, _ := sp.Style(); s == "" {
		fmt.Printf("STYLE.md is not written yet; the skill has the agent write it on first use\n(or do it now with `ghostwriter prompt style -p %s`).\n", sp.Name)
	}
	if _, err := lookPathSelf(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: `ghostwriter` is not on your PATH, so the agent won't be able to run it.")
	}
	return nil
}

// lookPathSelf checks the agent will find this binary by name.
func lookPathSelf() (string, error) {
	for dir := range strings.SplitSeq(os.Getenv("PATH"), string(os.PathListSeparator)) {
		p := filepath.Join(dir, "ghostwriter")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("not found")
}

var stdout = os.Stdout

// plural formats a count with its noun: "1 sample", "3 samples".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
