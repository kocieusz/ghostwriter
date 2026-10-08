// Package ingest reads writing samples from disk verbatim. Nothing is
// corrected: the writer's typos, odd capitals and run-ons are the fingerprint.
package ingest

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Extensions lists the file types Read understands.
var Extensions = []string{".txt", ".md", ".markdown", ".text", ".docx", ".pdf", ".eml"}

// Supported reports whether path has an extension Read understands.
func Supported(path string) bool {
	return slices.Contains(Extensions, strings.ToLower(filepath.Ext(path)))
}

// Read extracts the text of a sample. "-" reads stdin.
func Read(path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		return Clean(string(b)), err
	}
	var (
		text string
		err  error
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx":
		text, err = readDocx(path)
	case ".pdf":
		text, err = readPDF(path)
	case ".eml":
		text, err = readEML(path)
	default:
		var b []byte
		b, err = os.ReadFile(path)
		text = string(b)
	}
	if err != nil {
		return "", err
	}
	return Clean(text), nil
}

var bigGap = regexp.MustCompile(`\n{3,}`)

// Clean normalises line endings and whitespace only.
func Clean(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(bigGap.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// readDocx pulls paragraph text out of word/document.xml.
func readDocx(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		return docxText(rc)
	}
	return "", errors.New("not a Word document (no word/document.xml)")
}

func docxText(r io.Reader) (string, error) {
	dec := xml.NewDecoder(r)
	var out, para strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "t":
				inText = true
			case "tab":
				para.WriteByte('\t')
			case "br", "cr":
				para.WriteByte('\n')
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "p":
				out.WriteString(para.String())
				out.WriteString("\n\n")
				para.Reset()
			}
		case xml.CharData:
			if inText {
				para.Write(t)
			}
		}
	}
	return out.String(), nil
}

// readPDF shells out to poppler's pdftotext, the one dependency ghostwriter
// does not bundle; PDF text extraction is too large a job to do well inline.
func readPDF(path string) (string, error) {
	bin, err := exec.LookPath("pdftotext")
	if err != nil {
		return "", errors.New("PDF samples need pdftotext (brew install poppler / apt install poppler-utils), or save the file as .txt")
	}
	var out, stderr bytes.Buffer
	cmd := exec.Command(bin, "-enc", "UTF-8", path, "-")
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out.String(), nil
}

// readEML keeps the body of a plain-text email and drops its headers and any
// quoted reply, which are not the writer's words.
func readEML(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	if _, body, ok := strings.Cut(s, "\n\n"); ok {
		s = body
	}
	return StripQuotedReply(s), nil
}

var replyHeader = regexp.MustCompile(`(?m)^On .{5,200} wrote:\s*$|^-----Original Message-----|^From: .+\nSent: `)

// StripQuotedReply cuts an email at the start of the quoted thread and drops
// "> " quoted lines.
func StripQuotedReply(s string) string {
	if loc := replyHeader.FindStringIndex(s); loc != nil {
		s = s[:loc[0]]
	}
	var keep []string
	for l := range strings.SplitSeq(s, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(l), ">") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

var genreKeywords = []struct{ kw, genre string }{
	{"email", "email"}, {"mail", "email"}, {"letter", "email"}, {"message", "email"},
	{"essay", "essay"}, {"report", "report"}, {"memo", "report"},
	{"article", "article"}, {"blog", "article"}, {"post", "article"}, {"newsletter", "article"},
	{"answer", "answer"}, {"reply", "answer"}, {"comment", "answer"}, {"review", "answer"},
	{"slack", "chat"}, {"chat", "chat"}, {"tweet", "social"}, {"linkedin", "social"},
}

// Genre infers a genre from a filename keyword, or "other".
func Genre(path string) string {
	if strings.ToLower(filepath.Ext(path)) == ".eml" {
		return "email"
	}
	low := strings.ToLower(filepath.Base(path))
	for _, g := range genreKeywords {
		if strings.Contains(low, g.kw) {
			return g.genre
		}
	}
	return "other"
}
