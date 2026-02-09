package parser

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
)

// Parser converts raw document bytes into a DocTree.
type Parser interface {
	Parse(r io.Reader, filename string) (*doctree.DocTree, error)
}

// SupportedExtensions lists file extensions this service can handle.
var SupportedExtensions = map[string]bool{
	".txt": true,
	".md":  true,
	".csv": true,
	".html": true,
	".htm":  true,
	".pdf":  true,
	".docx": true,
}

// ForFile returns the appropriate parser for a filename.
func ForFile(filename string) (Parser, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt":
		return &TextParser{}, nil
	case ".md", ".markdown":
		return &MarkdownParser{}, nil
	case ".csv":
		return &CSVParser{}, nil
	case ".html", ".htm":
		return &HTMLParser{}, nil
	case ".pdf":
		return &PDFParser{}, nil
	case ".docx":
		return &DOCXParser{}, nil
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
}

// IsSupportedExtension checks if a file extension is supported.
func IsSupportedExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return SupportedExtensions[ext]
}
