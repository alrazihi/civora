package sanitizer

import (
	"regexp"
)

// PIISanitizer is implemented by types that redact personally identifiable
// information from document content before forwarding it to a provider.
//
// Redaction patterns are applied to text-like content only. Binary content
// types (images, archives, etc.) are returned unchanged because regex-based
// redaction would corrupt them.
var textContentTypes = map[string]bool{
	"text/plain":             true,
	"text/csv":               true,
	"application/json":       true,
	"application/xml":        true,
	"text/xml":               true,
	"text/markdown":          true,
	"application/javascript": true,
}

// RedactingSanitizer replaces common PII patterns with [REDACTED].
type RedactingSanitizer struct{}

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),                                // SSN
	regexp.MustCompile(`\b(?:\d[ -]*?){13,16}\b`),                              // credit card
	regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`), // email
	regexp.MustCompile(`\b\d{3}-\d{3}-\d{4}\b`),                                // phone
}

func (s *RedactingSanitizer) SanitizeDocumentContent(content []byte, contentType, fileName string) ([]byte, error) {
	if !textContentTypes[contentType] {
		return content, nil
	}
	out := content
	for _, p := range patterns {
		out = p.ReplaceAll(out, []byte("[REDACTED]"))
	}
	return out, nil
}

func NewRedactingSanitizer() *RedactingSanitizer {
	return &RedactingSanitizer{}
}
