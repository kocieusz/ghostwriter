package ingest

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadDocx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "essay.docx")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(`<?xml version="1.0"?><w:document xmlns:w="x"><w:body>` +
		`<w:p><w:r><w:t>First para,</w:t></w:r><w:r><w:t xml:space="preserve"> i wrote it.</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>Second one.</w:t></w:r></w:p></w:body></w:document>`))
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	text, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != "First para, i wrote it.\n\nSecond one." {
		t.Fatalf("got %q", text)
	}
}

func TestReadEMLDropsHeadersAndQuotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.eml")
	body := "From: me\r\nSubject: hi\r\n\r\nSure, friday works!\r\n\r\nOn Mon, 1 Jan 2024 at 10:00, Bob <b@x> wrote:\r\n> can you do friday?\r\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	text, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if text != "Sure, friday works!" {
		t.Fatalf("got %q", text)
	}
}

func TestGenre(t *testing.T) {
	for in, want := range map[string]string{
		"email_to_landlord.txt": "email", "Uni_Essay_2024.docx": "essay", "blog-post.md": "article",
		"random.txt": "other", "thread.eml": "email",
	} {
		if got := Genre(in); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	if got := Clean("a  \r\nb\n\n\n\nc  "); got != "a\nb\n\nc" || strings.Contains(got, "\r") {
		t.Fatalf("got %q", got)
	}
}
