package extract

import (
	"regexp"
	"strings"
)

// Fact represents an extracted fact from a document chunk.
type Fact struct {
	Text       string   `json:"text"`
	Category   string   `json:"category"`
	Entity     string   `json:"entity"`
	Topics     []string `json:"topics"`
	Salience   float64  `json:"salience"`
	Supersedes []string `json:"supersedes"`
	MinTrust   int      `json:"min_trust"`
}

var validCategories = map[string]bool{
	"entity_fact":     true,
	"preference":      true,
	"topic_knowledge": true,
	"procedure":       true,
}

// CategoryInfo maps category to (path template, memory type, default salience).
type CategoryInfo struct {
	PathTemplate string
	MemoryType   string
	DefaultSal   float64
}

var CategoryMap = map[string]CategoryInfo{
	"entity_fact":     {PathTemplate: "entities/{entity}/facts", MemoryType: "semantic", DefaultSal: 0.7},
	"preference":      {PathTemplate: "entities/{entity}/preferences", MemoryType: "semantic", DefaultSal: 0.8},
	"topic_knowledge": {PathTemplate: "topics/{topic}", MemoryType: "semantic", DefaultSal: 0.5},
	"procedure":       {PathTemplate: "procedures/{topic}", MemoryType: "procedural", DefaultSal: 0.6},
}

var injectionPattern = regexp.MustCompile(
	`(?i)(ignore\s+(previous|all|above)|system\s*prompt|you\s+are\s+now|` +
		`act\s+as\s+|pretend\s+|forget\s+(everything|all)|override|` +
		`new\s+instructions)`,
)

// ValidateFact checks a fact for validity. Returns true if valid.
func ValidateFact(f *Fact) bool {
	if f == nil {
		return false
	}
	text := strings.TrimSpace(f.Text)
	if len(text) < 3 || len(text) > 300 {
		return false
	}
	if !validCategories[f.Category] {
		return false
	}
	if injectionPattern.MatchString(text) {
		return false
	}
	if f.Salience < 0.01 || f.Salience > 1.0 {
		return false
	}
	// Clamp min_trust.
	if f.MinTrust < 0 || f.MinTrust > 10 {
		f.MinTrust = 0
	}
	// Limit topics to 3.
	if len(f.Topics) > 3 {
		f.Topics = f.Topics[:3]
	}
	return true
}

// Slugify converts a string to a URL/path-safe slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(s, "-")
	s = regexp.MustCompile(`-+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}
