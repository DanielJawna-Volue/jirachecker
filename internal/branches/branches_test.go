package branches

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		want []string // nil means "not a feature branch"
	}{
		{"feature/ABC-123-add-login", []string{"ABC-123"}},
		{"feat/abc-123_add-login", []string{"ABC-123"}},
		{"bug/Abc-7", []string{"ABC-7"}},
		{"Feature/ABC-1", []string{"ABC-1"}},
		{"feature/team/PROJ-42-thing", []string{"PROJ-42"}},
		{"feature/ABC-1-and-abc-1-and-XY2-30", []string{"ABC-1", "XY2-30"}},
		{"feature/cleanup", []string{}},
		{"main", nil},
		{"release/1.2", nil},
		{"feature/", nil},
	}
	for _, tt := range tests {
		got := Parse(tt.name, DefaultPrefixes)
		if tt.want == nil {
			if got != nil {
				t.Errorf("Parse(%q) = %+v, want nil", tt.name, got)
			}
			continue
		}
		if got == nil {
			t.Errorf("Parse(%q) = nil, want keys %v", tt.name, tt.want)
			continue
		}
		if len(got.IssueKeys) != 0 || len(tt.want) != 0 {
			if !reflect.DeepEqual(got.IssueKeys, tt.want) {
				t.Errorf("Parse(%q).IssueKeys = %v, want %v", tt.name, got.IssueKeys, tt.want)
			}
		}
	}
}

func TestExtractIssueKeysRejectsGluedPrefix(t *testing.T) {
	if got := ExtractIssueKeys("1ABC-2"); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}
