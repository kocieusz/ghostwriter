// Package store keeps ghostwriter's state self-contained under one directory
// (~/.ghostwriter by default):
//
//	~/.ghostwriter/profiles/<name>/corpus.jsonl   samples, verbatim
//	~/.ghostwriter/profiles/<name>/profile.json   the fingerprint
//	~/.ghostwriter/profiles/<name>/STYLE.md       the style guide your agent writes
package store

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kocieusz/ghostwriter/internal/voice"
)

// Home resolves the state directory: $GHOSTWRITER_HOME, else ~/.ghostwriter.
func Home() (string, error) {
	if h := os.Getenv("GHOSTWRITER_HOME"); h != "" {
		return ExpandHome(h)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ghostwriter"), nil
}

// ExpandHome expands a leading ~ and makes the path absolute.
func ExpandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}

var validName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// Profile is one named voice on disk.
type Profile struct {
	Name string
	Dir  string
}

// Open returns the profile directory for name without creating it.
func Open(name string) (Profile, error) {
	if !validName.MatchString(name) {
		return Profile{}, fmt.Errorf("invalid profile name %q: use lowercase letters, digits, '.', '_' or '-'", name)
	}
	home, err := Home()
	if err != nil {
		return Profile{}, err
	}
	return Profile{Name: name, Dir: filepath.Join(home, "profiles", name)}, nil
}

// List returns every profile name.
func List() ([]string, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(home, "profiles"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (p Profile) CorpusPath() string  { return filepath.Join(p.Dir, "corpus.jsonl") }
func (p Profile) ProfilePath() string { return filepath.Join(p.Dir, "profile.json") }
func (p Profile) StylePath() string   { return filepath.Join(p.Dir, "STYLE.md") }

// Exists reports whether the profile directory exists.
func (p Profile) Exists() bool {
	fi, err := os.Stat(p.Dir)
	return err == nil && fi.IsDir()
}

// Corpus loads the samples, or none if the corpus doesn't exist yet.
func (p Profile) Corpus() ([]voice.Doc, error) {
	f, err := os.Open(p.CorpusPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var docs []voice.Doc
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var d voice.Doc
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			return nil, fmt.Errorf("%s: %w", p.CorpusPath(), err)
		}
		docs = append(docs, d)
	}
	return docs, sc.Err()
}

// SaveCorpus writes the samples atomically.
func (p Profile) SaveCorpus(docs []voice.Doc) error {
	var b strings.Builder
	for _, d := range docs {
		line, err := json.Marshal(d)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return writeAtomic(p.CorpusPath(), []byte(b.String()))
}

// Load reads profile.json.
func (p Profile) Load() (*voice.Profile, error) {
	b, err := os.ReadFile(p.ProfilePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("profile %q has no fingerprint yet; run `ghostwriter analyze -p %s`", p.Name, p.Name)
	}
	if err != nil {
		return nil, err
	}
	var vp voice.Profile
	if err := json.Unmarshal(b, &vp); err != nil {
		return nil, fmt.Errorf("%s: %w", p.ProfilePath(), err)
	}
	if vp.Version != voice.ProfileVersion {
		return nil, fmt.Errorf("profile %q was built by another ghostwriter version; run `ghostwriter analyze -p %s` to rebuild it", p.Name, p.Name)
	}
	return &vp, nil
}

// Save writes profile.json.
func (p Profile) Save(vp *voice.Profile) error {
	b, err := json.MarshalIndent(vp, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(p.ProfilePath(), append(b, '\n'))
}

// Style returns STYLE.md, or "" if the writer's agent hasn't authored it yet.
func (p Profile) Style() (string, error) {
	b, err := os.ReadFile(p.StylePath())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return string(b), err
}

// Hash fingerprints sample text so re-adding a file replaces it instead of
// duplicating it.
func Hash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
