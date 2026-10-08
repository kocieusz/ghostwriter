package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kocieusz/ghostwriter/internal/voice"
)

func TestProfileRoundTrip(t *testing.T) {
	t.Setenv("GHOSTWRITER_HOME", t.TempDir())
	p, err := Open("work")
	if err != nil {
		t.Fatal(err)
	}
	if p.Exists() {
		t.Fatal("profile exists before anything was saved")
	}
	if docs, err := p.Corpus(); err != nil || docs != nil {
		t.Fatalf("empty corpus: %v %v", docs, err)
	}
	if _, err := p.Load(); err == nil {
		t.Fatal("loading an unanalyzed profile should fail")
	}

	text := "Line one, with \"quotes\" and ünïcode.\n\nSecond paragraph."
	docs := []voice.Doc{{Name: "a.txt", Genre: "email", Words: 9, Hash: Hash(text), Text: text}}
	if err := p.SaveCorpus(docs); err != nil {
		t.Fatal(err)
	}
	got, err := p.Corpus()
	if err != nil || len(got) != 1 || got[0].Text != text {
		t.Fatalf("corpus round trip: %+v %v", got, err)
	}

	vp := voice.Build(docs)
	if err := p.Save(vp); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Load(); err != nil {
		t.Fatal(err)
	}
	names, err := List()
	if err != nil || len(names) != 1 || names[0] != "work" {
		t.Fatalf("list: %v %v", names, err)
	}
	// no temp files left behind by the atomic writes
	entries, _ := os.ReadDir(p.Dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == "" && e.Name()[0] == '.' {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestLoadRejectsOldVersion(t *testing.T) {
	t.Setenv("GHOSTWRITER_HOME", t.TempDir())
	p, _ := Open("me")
	if err := p.Save(&voice.Profile{Version: voice.ProfileVersion + 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Load(); err == nil {
		t.Fatal("a profile from another version should ask for analyze")
	}
}

func TestOpenValidatesName(t *testing.T) {
	for _, bad := range []string{"", "Work", "../x", "-x", "a b"} {
		if _, err := Open(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := Open("work.v2_x-1"); err != nil {
		t.Errorf("valid name rejected: %v", err)
	}
}
