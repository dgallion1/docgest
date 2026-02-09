package parser

import (
	"fmt"
	"io"
	"strings"

	"github.com/dgallion1/docgest/internal/doctree"
	"golang.org/x/net/html"
)

// HTMLParser handles HTML files.
type HTMLParser struct{}

func (p *HTMLParser) Parse(r io.Reader, filename string) (*doctree.DocTree, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	tree := &doctree.DocTree{
		Title: strings.TrimSuffix(strings.TrimSuffix(filename, ".html"), ".htm"),
	}

	// Extract title from <title> tag if present.
	if title := findTitle(doc); title != "" {
		tree.Title = title
	}

	// Walk the HTML and build tree from heading tags.
	type stackEntry struct {
		node  *doctree.DocNode
		level int
	}
	root := &doctree.DocNode{Title: tree.Title}
	stack := []stackEntry{{node: root, level: 0}}
	var currentText strings.Builder

	flushText := func() {
		t := strings.TrimSpace(currentText.String())
		if t != "" {
			top := stack[len(stack)-1].node
			if top.Text != "" {
				top.Text += "\n\n" + t
			} else {
				top.Text = t
			}
		}
		currentText.Reset()
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			level := headingLevel(n.Data)
			if level > 0 {
				flushText()
				title := textContent(n)

				newNode := &doctree.DocNode{Title: title}
				for len(stack) > 1 && stack[len(stack)-1].level >= level {
					stack = stack[:len(stack)-1]
				}
				parent := stack[len(stack)-1].node
				parent.Children = append(parent.Children, newNode)
				stack = append(stack, stackEntry{node: newNode, level: level})
				return // Don't recurse into heading children (already extracted text).
			}

			// Skip non-content elements.
			switch n.Data {
			case "script", "style", "nav", "footer", "header":
				return
			case "p", "li", "td", "blockquote":
				t := textContent(n)
				if t != "" {
					if currentText.Len() > 0 {
						currentText.WriteString("\n\n")
					}
					currentText.WriteString(t)
				}
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	// Find <body> or use whole document.
	body := findBody(doc)
	if body != nil {
		walk(body)
	} else {
		walk(doc)
	}
	flushText()

	tree.Children = root.Children
	if len(tree.Children) == 0 && root.Text != "" {
		tree.Children = []*doctree.DocNode{{Text: root.Text}}
	}

	return tree, nil
}

func headingLevel(tag string) int {
	switch tag {
	case "h1":
		return 1
	case "h2":
		return 2
	case "h3":
		return 3
	case "h4":
		return 4
	case "h5":
		return 5
	case "h6":
		return 6
	}
	return 0
}

func textContent(n *html.Node) string {
	var buf strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return strings.TrimSpace(buf.String())
}

func findTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		return textContent(n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findTitle(c); t != "" {
			return t
		}
	}
	return ""
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if b := findBody(c); b != nil {
			return b
		}
	}
	return nil
}
