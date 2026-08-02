package sync

import (
	"reflect"
	"testing"
)

func TestNewIgnoreRulesNormalization(t *testing.T) {
	raw := []string{"  node_modules ", "//.git//", ".", "", "build", "node_modules"}
	got := NewIgnoreRules(raw)
	want := []string{"node_modules", ".git", "build"}
	if !reflect.DeepEqual(got.patterns, want) {
		t.Fatalf("NewIgnoreRules(%v) = %v, want %v", raw, got.patterns, want)
	}
}

func TestNewIgnoreRulesEmpty(t *testing.T) {
	rules := NewIgnoreRules(nil)
	if !rules.Empty() {
		t.Fatal("empty rules must report Empty()")
	}
	if rules.Matches("anything") {
		t.Fatal("empty rules must not match anything")
	}
}

func TestIgnoreRulesMatches(t *testing.T) {
	rules := NewIgnoreRules([]string{"node_modules", ".git", "a/b"})

	tests := []struct {
		rel  string
		want bool
	}{
		{rel: "node_modules", want: true},
		{rel: "node_modules/pkg/index.js", want: true},
		{rel: "src/node_modules/x", want: true},
		{rel: "src/app/node_modules", want: true},
		{rel: ".git/config", want: true},
		{rel: "src/.git", want: true},
		{rel: "a/b", want: true},
		{rel: "a/b/c.txt", want: true},
		{rel: "x/a/b", want: false},
		{rel: "node_modulesx", want: false},
		{rel: "src/app.ts", want: false},
		{rel: "src/app/", want: false},
		{rel: ".", want: false},
		{rel: "..", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			if got := rules.Matches(tt.rel); got != tt.want {
				t.Fatalf("IgnoreRules.Matches(%q) = %v, want %v", tt.rel, got, tt.want)
			}
		})
	}
}
