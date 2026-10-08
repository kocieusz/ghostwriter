package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func vars() SkillVars {
	return SkillVars{SkillName: DefaultName, Writer: "Kasia", Profile: "kasia", Genres: "email, essay", MaxPasses: 4}
}

func TestInstall(t *testing.T) {
	dir := t.TempDir()
	dst, err := Install(dir, vars(), map[string]string{"cli.md": "# manual"})
	if err != nil {
		t.Fatal(err)
	}
	if dst != filepath.Join(dir, "ghostwriter") {
		t.Fatalf("installed to %s", dst)
	}
	b, _ := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	for _, want := range []string{"name: ghostwriter\n", "ghostwriter context -p kasia", "--source brief.md", "at most 4 passes", "references/tells.md"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("SKILL.md missing %q", want)
		}
	}
	ref, err := os.ReadFile(filepath.Join(dst, "references", "tells.md"))
	if err != nil || !strings.Contains(string(ref), "`negative_parallelism`") {
		t.Fatalf("references/tells.md missing or incomplete: %v", err)
	}
	// reinstalling over our own skill is fine
	if _, err := Install(dir, vars(), map[string]string{"cli.md": "# manual"}); err != nil {
		t.Fatal(err)
	}
	// a foreign skill with the same name is left alone
	if err := os.WriteFile(filepath.Join(dst, "SKILL.md"), []byte("---\nname: ghostwriter\ndescription: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(dir, vars(), map[string]string{"cli.md": "# manual"}); err == nil {
		t.Fatal("overwrote a skill ghostwriter didn't write")
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"ghostwriter", "write-as-kasia", "a", "x1-2y"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Ghostwriter", "-gw", "gw-", "write--as", "write_as", "write.as", strings.Repeat("a", 65)} {
		if ValidateName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// A writer name full of YAML syntax must still produce a valid header.
func TestFrontmatterQuoting(t *testing.T) {
	v := vars()
	v.Writer = `Kasia: "the PM" # really`
	out, err := RenderSkill(v)
	if err != nil {
		t.Fatal(err)
	}
	fm, err := frontmatter(out)
	if err != nil {
		t.Fatal(err)
	}
	if fm["name"] != "ghostwriter" || !strings.Contains(fm["description"], `Kasia: "the PM" # really's personal`) {
		t.Fatalf("frontmatter parsed wrong: %q", fm)
	}
}

func TestDescriptionTooLong(t *testing.T) {
	v := vars()
	v.Writer = strings.Repeat("x", 400)
	if _, err := RenderSkill(v); err == nil {
		t.Fatal("over-long description accepted")
	}
}
