package cmd

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	withMain := func(v string) *debug.BuildInfo {
		return &debug.BuildInfo{Main: debug.Module{Version: v}}
	}
	tests := []struct {
		name string
		v    string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"ldflags version wins", "0.39.0", withMain("v0.40.0"), true, "0.39.0"},
		{"go install module version", "dev", withMain("v0.39.1"), true, "0.39.1"},
		{"pseudo version", "dev", withMain("v0.39.1-0.20260923120000-abcdef123456"), true, "0.39.1-0.20260923120000-abcdef123456"},
		{"local build keeps dev", "dev", withMain("(devel)"), true, "dev"},
		{"dirty build keeps dev", "dev", withMain("v0.39.0+dirty"), true, "dev"},
		{"empty module version keeps dev", "dev", withMain(""), true, "dev"},
		{"no build info keeps dev", "dev", nil, false, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.v, tt.info, tt.ok); got != tt.want {
				t.Errorf("resolveVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
