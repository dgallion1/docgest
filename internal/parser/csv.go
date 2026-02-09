package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
)

// CSVParser handles CSV files.
type CSVParser struct{}

func (p *CSVParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(filename, ".csv"),
	}

	if len(records) == 0 {
		return tree, nil
	}

	// First row is headers.
	headers := records[0]

	// Group rows into batches of 20 for manageable chunks.
	const batchSize = 20
	dataRows := records[1:]

	for i := 0; i < len(dataRows); i += batchSize {
		end := i + batchSize
		if end > len(dataRows) {
			end = len(dataRows)
		}
		batch := dataRows[i:end]

		var text strings.Builder
		text.WriteString("Headers: " + strings.Join(headers, ", ") + "\n\n")
		for _, row := range batch {
			for j, cell := range row {
				if j < len(headers) {
					text.WriteString(headers[j] + ": " + cell)
				} else {
					text.WriteString(cell)
				}
				if j < len(row)-1 {
					text.WriteString(", ")
				}
			}
			text.WriteString("\n")
		}

		tree.Children = append(tree.Children, &doctree.DocNode{
			Title: fmt.Sprintf("Rows %d-%d", i+2, end+1), // 1-indexed, skip header
			Text:  text.String(),
		})
	}

	return tree, nil
}
