package tmpl

import (
	"fmt"
	"sort"
)

// Segment is one piece of a parsed template: either literal text or a
// parameter placeholder. Exactly one of Literal and Param is meaningful;
// Param being non-empty marks the segment as a placeholder.
type Segment struct {
	Literal string
	Param   string
}

// Parse parses template text left to right.
//
// In plain text, "{{" and "}}" each produce one literal brace. A single "{"
// begins a placeholder, which starts with an ASCII letter, continues with
// ASCII letters, digits or underscores, and ends at the first "}". Any other
// unpaired brace is illegal.
func Parse(text string) ([]Segment, error) {
	var segs []Segment
	var literal []rune

	flush := func() {
		if len(literal) > 0 {
			segs = append(segs, Segment{Literal: string(literal)})
			literal = nil
		}
	}

	runes := []rune(text)
	for i := 0; i < len(runes); {
		switch runes[i] {
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
			segs = append(segs, Segment{Param: name})
			i = next
		case '}':
			if i+1 < len(runes) && runes[i+1] == '}' {
				literal = append(literal, '}')
				i += 2
				continue
			}
			return nil, fmt.Errorf("unmatched '}' at byte %d (use \"}}\" for a literal brace)", indexToByte(runes, i))
		default:
			literal = append(literal, runes[i])
			i++
		}
	}
	flush()
	return segs, nil
}

func parsePlaceholder(runes []rune, start int) (string, int, error) {
	i := start + 1 // consume "{"
	if i >= len(runes) || !isLetter(runes[i]) {
		return "", 0, fmt.Errorf("invalid placeholder at byte %d: name must start with a letter", indexToByte(runes, start))
	}
	nameStart := i
	i++
	for i < len(runes) && isNameRune(runes[i]) {
		i++
	}
	name := string(runes[nameStart:i])
	if i >= len(runes) {
		return "", 0, fmt.Errorf("invalid placeholder %q at byte %d: missing closing '}'", "{"+name, indexToByte(runes, start))
	}
	if runes[i] != '}' {
		return "", 0, fmt.Errorf("invalid placeholder at byte %d: %q is not allowed in a name (letters, digits and underscore only)", indexToByte(runes, start), string(runes[i]))
	}
	return name, i + 1, nil // consume "}"
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func isNameRune(r rune) bool {
	return isLetter(r) || r >= '0' && r <= '9' || r == '_'
}

func indexToByte(runes []rune, index int) int {
	return len(string(runes[:index]))
}

// Placeholders returns the set of placeholder names appearing in the
// segments.
func Placeholders(segs []Segment) map[string]bool {
	set := make(map[string]bool)
	for _, seg := range segs {
		if seg.Param != "" {
			set[seg.Param] = true
		}
	}
	return set
}

// SamePlaceholders reports whether both segments use the same placeholder
// name set.
func SamePlaceholders(a, b []Segment) bool {
	sa, sb := Placeholders(a), Placeholders(b)
	if len(sa) != len(sb) {
		return false
	}
	for name := range sa {
		if !sb[name] {
			return false
		}
	}
	return true
}

// Render substitutes string parameters into the parsed segments. Parameter
// values are inserted verbatim and never interpreted as templates.
func Render(segs []Segment, args map[string]string) string {
	var out []rune
	for _, seg := range segs {
		if seg.Param != "" {
			out = append(out, []rune(args[seg.Param])...)
		} else {
			out = append(out, []rune(seg.Literal)...)
		}
	}
	return string(out)
}

// SortedNames returns placeholder names in deterministic order.
func SortedNames(set map[string]bool) []string {
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
