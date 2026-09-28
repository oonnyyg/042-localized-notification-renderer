package msg

import (
	"fmt"
	"unicode/utf8"
)

// Segment is one parsed piece of a message template.
// When Name is empty it is a literal Text fragment, otherwise it is a
// parameter placeholder identified by Name.
type Segment struct {
	Text string `json:"text,omitempty"`
	Name string `json:"name,omitempty"`
}

// IsPlaceholder reports whether the segment is a placeholder.
func (s Segment) IsPlaceholder() bool { return s.Name != "" }

// Parse parses a message template from left to right.
//
// Rules:
//   - "{{" renders one literal "{"; "}}" renders one literal "}".
//   - A single "{" starts a placeholder. The name must begin with an ASCII
//     letter and may contain only letters, digits and underscores. It is
//     terminated by the first "}".
//   - A placeholder name may be used more than once.
//   - Any other unpaired brace is illegal.
func Parse(text string) ([]Segment, error) {
	var segments []Segment
	var literal []rune

	flush := func() {
		if len(literal) > 0 {
			segments = append(segments, Segment{Text: string(literal)})
			literal = literal[:0]
		}
	}

	runes := []rune(text)
	for i := 0; i < len(runes); {
		r := runes[i]
		switch r {
		case '{':
			if i+1 < len(runes) && runes[i+1] == '{' {
				literal = append(literal, '{')
				i += 2
				continue
			}
			name, next, err := parsePlaceholder(runes, i)
			if err != nil {
				return nil, err
			}
			flush()
			segments = append(segments, Segment{Name: name})
			i = next
		case '}':
			if i+1 < len(runes) && runes[i+1] == '}' {
				literal = append(literal, '}')
				i += 2
				continue
			}
			return nil, fmt.Errorf("at byte %d: unmatched '}'", byteOffset(runes, i))
		default:
			literal = append(literal, r)
			i++
		}
	}
	flush()
	return segments, nil
}

func parsePlaceholder(runes []rune, start int) (string, int, error) {
	i := start + 1 // skip '{'
	if i >= len(runes) || !isLetter(runes[i]) {
		return "", 0, fmt.Errorf("at byte %d: placeholder name must start with a letter", byteOffset(runes, start))
	}
	nameStart := i
	for i < len(runes) && isNameRune(runes[i]) {
		i++
	}
	if i >= len(runes) {
		return "", 0, fmt.Errorf("at byte %d: placeholder is missing closing '}'", byteOffset(runes, start))
	}
	if runes[i] != '}' {
		return "", 0, fmt.Errorf("at byte %d: illegal character %q in placeholder name", byteOffset(runes, i), runes[i])
	}
	return string(runes[nameStart:i]), i + 1, nil
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNameRune(r rune) bool {
	return isLetter(r) || r >= '0' && r <= '9' || r == '_'
}

func byteOffset(runes []rune, idx int) int {
	off := 0
	for j := 0; j < idx; j++ {
		off += utf8.RuneLen(runes[j])
	}
	return off
}

// Placeholders returns the set of placeholder names in parsed segments.
func Placeholders(segments []Segment) map[string]struct{} {
	set := make(map[string]struct{})
	for _, seg := range segments {
		if seg.IsPlaceholder() {
			set[seg.Name] = struct{}{}
		}
	}
	return set
}
