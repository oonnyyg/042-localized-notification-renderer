package tmpl

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Segment
	}{
		{
			name: "plain text",
			text: "hello world",
			want: []Segment{{Literal: "hello world"}},
		},
		{
			name: "single placeholder",
			text: "Hello {name}!",
			want: []Segment{
				{Literal: "Hello "},
				{Param: "name"},
				{Literal: "!"},
			},
		},
		{
			name: "escaped double braces",
			text: "{{code}} and {{{x}}}",
			want: []Segment{
				{Literal: "{code} and {"},
				{Param: "x"},
				{Literal: "}"},
			},
		},
		{
			name: "repeated placeholder",
			text: "{a}-{a}-{b1}",
			want: []Segment{
				{Param: "a"},
				{Literal: "-"},
				{Param: "a"},
				{Literal: "-"},
				{Param: "b1"},
			},
		},
		{
			name: "underscore and digits",
			text: "{user_name_2}",
			want: []Segment{{Param: "user_name_2"}},
		},
		{
			name: "empty input",
			text: "",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.text)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tt.text, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.text, got, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"}",
		"hello }",
		"{1name}",
		"{na-me}",
		"{name",
		"{}",
		"{ name}",
	}
	for _, text := range bad {
		if _, err := Parse(text); err == nil {
			t.Errorf("Parse(%q) expected error, got nil", text)
		}
	}
}

func TestRenderDoesNotInterpretBracesInArgs(t *testing.T) {
	segs, err := Parse("x={value}")
	if err != nil {
		t.Fatal(err)
	}
	got := Render(segs, map[string]string{"value": "{evil}{{also}}"})
	if got != "x={evil}{{also}}" {
		t.Fatalf("got %q", got)
	}
}

func TestSamePlaceholders(t *testing.T) {
	a, _ := Parse("{x}-{y}-{x}")
	b, _ := Parse("{y}{x}")
	c, _ := Parse("{x}")
	if !SamePlaceholders(a, b) {
		t.Error("a and b should share the placeholder set")
	}
	if SamePlaceholders(a, c) {
		t.Error("a and c should differ")
	}
}
