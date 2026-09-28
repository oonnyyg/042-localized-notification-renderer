package msg

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []Segment
		wantErr bool
	}{
		{
			name:  "plain text",
			input: "hello world",
			want:  []Segment{{Text: "hello world"}},
		},
		{
			name:  "single placeholder",
			input: "Hi {name}",
			want: []Segment{
				{Text: "Hi "},
				{Name: "name"},
			},
		},
		{
			name:  "escaped braces",
			input: "Use {{curly}} brackets",
			want: []Segment{
				{Text: "Use {curly} brackets"},
			},
		},
		{
			name:  "escaped brace before placeholder",
			input: "{{{name}}}",
			want: []Segment{
				{Text: "{"},
				{Name: "name"},
				{Text: "}"},
			},
		},
		{
			name:  "repeated placeholder",
			input: "{n}-{n}-{n_1}",
			want: []Segment{
				{Name: "n"},
				{Text: "-"},
				{Name: "n"},
				{Text: "-"},
				{Name: "n_1"},
			},
		},
		{
			name:  "unicode text",
			input: "你好{city}🌏",
			want: []Segment{
				{Text: "你好"},
				{Name: "city"},
				{Text: "🌏"},
			},
		},
		{
			name:    "unmatched right brace",
			input:   "oops }",
			wantErr: true,
		},
		{
			name:    "missing closing brace",
			input:   "hi {name",
			wantErr: true,
		},
		{
			name:    "name starts with digit",
			input:   "{1bad}",
			wantErr: true,
		},
		{
			name:    "name with illegal char",
			input:   "{na-me}",
			wantErr: true,
		},
		{
			name:    "empty placeholder",
			input:   "{}",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlaceholders(t *testing.T) {
	segs, err := Parse("{a} x {b} x {a}")
	if err != nil {
		t.Fatal(err)
	}
	got := Placeholders(segs)
	want := map[string]struct{}{"a": {}, "b": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Placeholders = %v, want %v", got, want)
	}
}
