package extract

import (
	"fmt"
	"strings"
)

const ExtractionPrompt = `Extract structured facts from the following document section. Return a JSON array of facts. Each fact object must have these fields:

- "text": concise statement of the fact (string, max 200 chars)
- "category": one of "entity_fact", "preference", "topic_knowledge", "procedure"
- "entity": the person or thing this fact is about (string or null)
- "topics": list of topic slugs relevant to this fact (list of strings, max 3)
- "salience": importance from 0.1 to 1.0 (float)
- "supersedes": list of paths of existing memories this fact replaces (list of strings, default [])
- "min_trust": minimum trust level (integer 0-10) to retrieve this memory (default 0)

Rules:
- Only extract concrete, factual information — not opinions or speculation
- Prefer specific facts over vague generalizations
- Extract ONE fact per distinct attribute or trait
- The "text" field MUST name the entity it's about. Write "Milo plays fetch" not "plays fetch". Each fact should be understandable on its own.
- Entity names should be lowercase, no spaces (use underscores)
- Topic slugs should be lowercase, hyphenated
- Salience: personal facts=0.7, topic knowledge=0.5, procedures=0.6
- Default min_trust to 0. Most facts should be 0.
- Do NOT extract episode-type facts from documents
- Return an empty array [] if nothing worth remembering

Respond with ONLY the JSON array, no other text.`

// BuildChunkPrompt creates the full prompt for extracting facts from a chunk,
// including document title and section breadcrumb context.
func BuildChunkPrompt(docTitle string, breadcrumb []string, chunkText string) string {
	var sb strings.Builder
	sb.WriteString(ExtractionPrompt)
	sb.WriteString("\n\n---\n")
	sb.WriteString(fmt.Sprintf("Document: %q\n", docTitle))
	if len(breadcrumb) > 0 {
		sb.WriteString("Section: ")
		sb.WriteString(strings.Join(breadcrumb, " > "))
		sb.WriteString("\n")
	}
	sb.WriteString("---\n")
	sb.WriteString(chunkText)
	return sb.String()
}
