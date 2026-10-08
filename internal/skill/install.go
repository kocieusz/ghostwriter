package skill

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/kocieusz/ghostwriter/internal/tells"
)

// DefaultName is the skill's name and directory unless --name says otherwise.
const DefaultName = "ghostwriter"

// DefaultDir is where install puts skills: the project-local, harness-neutral
// skills directory, relative to the current directory.
var DefaultDir = filepath.Join(".agents", "skills")

// marker identifies a SKILL.md that ghostwriter wrote, so reinstalling may
// overwrite it but a hand-written skill with the same name is left alone.
const marker = "generated-by: ghostwriter"

// SkillVars fill SKILL.md.
type SkillVars struct {
	SkillName string
	Writer    string
	Profile   string
	Genres    string
	MaxPasses int
}

// nameRe is the Agent Skills name rule: lowercase letters, digits and single
// hyphens, not at either end.
var nameRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// ValidateName checks a skill name against the Agent Skills specification
// (https://agentskills.io/specification).
func ValidateName(name string) error {
	if name == "" || len(name) > 64 {
		return fmt.Errorf("skill name %q must be 1–64 characters", name)
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("skill name %q may only use lowercase letters, digits and single hyphens, not at the start or end", name)
	}
	return nil
}

// Frontmatter limits from the specification.
const (
	maxDescription   = 1024
	maxCompatibility = 500
)

// RenderSkill returns SKILL.md after checking it against the specification.
func RenderSkill(v SkillVars) (string, error) {
	if err := ValidateName(v.SkillName); err != nil {
		return "", err
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "SKILL.md.tmpl", v); err != nil {
		return "", err
	}
	out := b.String()
	fm, err := frontmatter(out)
	if err != nil {
		return "", err
	}
	if n := utf8.RuneCountInString(fm["description"]); n == 0 || n > maxDescription {
		return "", fmt.Errorf("description is %d characters; the spec allows 1–%d (shorten --writer)", n, maxDescription)
	}
	if n := utf8.RuneCountInString(fm["compatibility"]); n > maxCompatibility {
		return "", fmt.Errorf("compatibility is %d characters; the spec allows %d", n, maxCompatibility)
	}
	return out, nil
}

// frontmatter reads the top-level scalar fields of SKILL.md's YAML header.
// The template emits every string field JSON-quoted (valid YAML), so
// unquoting them is exact.
func frontmatter(doc string) (map[string]string, error) {
	rest, ok := strings.CutPrefix(doc, "---\n")
	if !ok {
		return nil, errors.New("SKILL.md must start with YAML frontmatter")
	}
	header, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		return nil, errors.New("SKILL.md frontmatter is not closed")
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(header, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok || strings.HasPrefix(line, " ") {
			continue
		}
		val = strings.TrimSpace(val)
		if strings.HasPrefix(val, `"`) {
			if err := json.Unmarshal([]byte(val), &val); err != nil {
				return nil, fmt.Errorf("frontmatter %s: %w", key, err)
			}
		}
		fields[key] = val
	}
	return fields, nil
}

// yamlString quotes s as a YAML double-quoted scalar. JSON string syntax is a
// subset of YAML's, so a writer name with ": " or quotes can't break the header.
func yamlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// Install writes the skill into dir/<name>/: SKILL.md, references/tells.md,
// and each of refs (file name → content) under references/. It refuses to
// overwrite a skill it did not write.
func Install(dir string, v SkillVars, refs map[string]string) (string, error) {
	body, err := RenderSkill(v)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, v.SkillName)
	path := filepath.Join(dst, "SKILL.md")
	if old, err := os.ReadFile(path); err == nil && !strings.Contains(string(old), marker) {
		return "", fmt.Errorf("%s already exists and was not written by ghostwriter; pick another --name", path)
	}
	if err := os.MkdirAll(filepath.Join(dst, "references"), 0o755); err != nil {
		return "", err
	}
	files := map[string]string{"tells.md": TellsReference()}
	maps.Copy(files, refs)
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dst, "references", name), []byte(body), 0o644); err != nil {
			return "", err
		}
	}
	return dst, os.WriteFile(path, []byte(body), 0o644)
}

// TellsReference documents every AI-tell rule for the agent to consult when a
// score note names one. It is loaded only on demand, keeping SKILL.md short.
func TellsReference() string {
	var b strings.Builder
	b.WriteString("# AI tells the scorer checks\n\n")
	b.WriteString("`ghostwriter score` flags these patterns by rule id. Each is judged against the\n")
	b.WriteString("writer's own rate: a pattern their samples use is allowed at that rate, one\n")
	b.WriteString("they never use costs points every time. Hard tells always block a pass. Weak\n")
	b.WriteString("tells only cost points when another tell is also over the writer's rate.\n\n")
	b.WriteString("Fix a flagged line by rewriting the sentence around its point, not by swapping\n")
	b.WriteString("one word; phrase-by-phrase patching tends to create new tells.\n\n")
	for _, s := range []tells.Strength{tells.Hard, tells.Strong, tells.Medium, tells.Weak} {
		fmt.Fprintf(&b, "## %s\n\n", strings.ToUpper(s.String()[:1])+s.String()[1:])
		for _, r := range tells.Rules {
			if r.Strength == s {
				fmt.Fprintf(&b, "- `%s`: %s. %s\n", r.ID, r.Name, r.Fix)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
