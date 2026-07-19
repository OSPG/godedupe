package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandHomeDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "home only",
			in:   "~",
			want: home,
		},
		{
			name: "home prefix",
			in:   "~/.godedupe-exclude",
			want: filepath.Join(home, ".godedupe-exclude"),
		},
		{
			name: "unchanged",
			in:   "./.godedupe-exclude",
			want: "./.godedupe-exclude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := expandHomeDir(tt.in)
			if err != nil {
				t.Fatalf("expandHomeDir() returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expandHomeDir(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLoadExcludePatternsExpandsDefaultExcludeFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	excludeFile := filepath.Join(home, ".godedupe-exclude")
	if err := os.WriteFile(excludeFile, []byte("*.tmp\ncache/\n"), 0644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	excludePatterns = nil
	t.Cleanup(func() {
		excludePatterns = nil
	})

	if err := loadExcludePatterns(defaultExcludeFile); err != nil {
		t.Fatalf("loadExcludePatterns() returned error: %v", err)
	}

	want := []string{"*.tmp", "cache/"}
	if !reflect.DeepEqual(excludePatterns, want) {
		t.Fatalf("excludePatterns = %#v, want %#v", excludePatterns, want)
	}
}
