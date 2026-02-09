package parser

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
	pdflib "github.com/ledongthuc/pdf"
)

// PDFParser handles PDF files. It tries the Go library first,
// then falls back to pdftotext if available.
type PDFParser struct {
	FallbackPdftotext bool
}

func (p *PDFParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	// ledongthuc/pdf requires a ReadSeeker+size, so we write to a temp file.
	tmp, err := os.CreateTemp("", "docgest-pdf-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	tmp.Close()

	text, err := extractPDFText(tmpPath)
	if err != nil && p.FallbackPdftotext {
		text, err = extractPdftotext(tmpPath)
	}
	if err != nil {
		return nil, fmt.Errorf("extract pdf text: %w", err)
	}

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(filename, ".pdf"),
	}

	// Split into pages (simple heuristic: form feed or large gaps).
	pages := splitPages(text)
	for i, page := range pages {
		page = strings.TrimSpace(page)
		if page == "" {
			continue
		}
		tree.Children = append(tree.Children, &doctree.DocNode{
			Title: fmt.Sprintf("Page %d", i+1),
			Text:  page,
			Page:  i + 1,
		})
	}

	if len(tree.Children) == 0 && strings.TrimSpace(text) != "" {
		tree.Children = []*doctree.DocNode{{Text: strings.TrimSpace(text), Page: 1}}
	}

	return tree, nil
}

func extractPDFText(path string) (string, error) {
	f, reader, err := pdflib.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf strings.Builder
	numPages := reader.NumPage()
	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		if i > 1 {
			buf.WriteString("\f") // Form feed as page separator.
		}
		buf.WriteString(text)
	}
	return buf.String(), nil
}

func extractPdftotext(path string) (string, error) {
	cmd := exec.Command("pdftotext", "-layout", path, "-")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return string(out), nil
}

func splitPages(text string) []string {
	return strings.Split(text, "\f")
}
