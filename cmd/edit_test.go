package cmd

import (
	"strconv"
	"strings"
	"testing"
)

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		items []string
		want  []string
	}{
		{"adds new items", []string{"a", "b"}, []string{"c"}, []string{"a", "b", "c"}},
		{"skips duplicates", []string{"a", "b"}, []string{"b", "c"}, []string{"a", "b", "c"}},
		{"empty slice", nil, []string{"a"}, []string{"a"}},
		{"no items", []string{"a"}, nil, []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUnique(tt.slice, tt.items...)
			assertStrings(t, got, tt.want)
		})
	}
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		items []string
		want  []string
	}{
		{"removes matching", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{"removes multiple", []string{"a", "b", "c"}, []string{"a", "c"}, []string{"b"}},
		{"no match", []string{"a", "b"}, []string{"x"}, []string{"a", "b"}},
		{"empty slice", nil, []string{"a"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeAll(tt.slice, tt.items...)
			assertStrings(t, got, tt.want)
		})
	}
}

func TestAppendUniqueInts(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		items []int
		want  []int
	}{
		{"adds new", []int{1, 2}, []int{3}, []int{1, 2, 3}},
		{"skips dupes", []int{1, 2}, []int{2, 3}, []int{1, 2, 3}},
		{"empty slice", nil, []int{1}, []int{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUniqueInts(tt.slice, tt.items...)
			assertInts(t, got, tt.want)
		})
	}
}

func TestApplyBodyReplacements(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		oldTexts []string
		newTexts []string
		want     string
		wantErr  string
	}{
		{
			name:     "replaces one exact passage",
			body:     "before\n[BLOCKED: design]\nafter",
			oldTexts: []string{"[BLOCKED: design]"},
			newTexts: []string{"[RESOLVED: design]"},
			want:     "before\n[RESOLVED: design]\nafter",
		},
		{
			name:     "applies repeated flag pairs in order",
			body:     "alpha beta gamma",
			oldTexts: []string{"alpha", "gamma"},
			newTexts: []string{"one", "three"},
			want:     "one beta three",
		},
		{
			name:     "rejects unequal pair counts",
			body:     "alpha",
			oldTexts: []string{"alpha"},
			wantErr:  "same number",
		},
		{
			name:     "rejects an empty search passage",
			body:     "alpha",
			oldTexts: []string{""},
			newTexts: []string{"beta"},
			wantErr:  "must not be empty",
		},
		{
			name:     "rejects a missing passage",
			body:     "alpha",
			oldTexts: []string{"beta"},
			newTexts: []string{"gamma"},
			wantErr:  "found 0 times",
		},
		{
			name:     "rejects an ambiguous passage",
			body:     "alpha plus alpha",
			oldTexts: []string{"alpha"},
			newTexts: []string{"beta"},
			wantErr:  "found 2 times",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyBodyReplacements(tt.body, tt.oldTexts, tt.newTexts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRemoveInts(t *testing.T) {
	tests := []struct {
		name  string
		slice []int
		items []int
		want  []int
	}{
		{"removes matching", []int{1, 2, 3}, []int{2}, []int{1, 3}},
		{"removes multiple", []int{1, 2, 3}, []int{1, 3}, []int{2}},
		{"no match", []int{1, 2}, []int{5}, []int{1, 2}},
		{"empty slice", nil, []int{1}, []int{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeInts(tt.slice, tt.items...)
			assertInts(t, got, tt.want)
		})
	}
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: got %v", len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, v, want[i])
		}
	}
}

func assertInts(t *testing.T, got, want []int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: got %v", len(got), len(want), got)
	}
	for i, v := range got {
		if v != want[i] {
			t.Errorf("got[%d] = %s, want %s", i, strconv.Itoa(v), strconv.Itoa(want[i]))
		}
	}
}
