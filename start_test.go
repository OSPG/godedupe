package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
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

func runGodedupeHelper(t *testing.T, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-test.run=TestDuplicateDirectoriesOnlyHelper", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "GODEDUPE_HELPER_PROCESS=1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run helper process: %v\n%s", err, out)
	}

	return string(out)
}

func outputHasLine(output, want string) bool {
	for _, line := range strings.Split(output, "\n") {
		if line == want {
			return true
		}
	}

	return false
}

func nonEmptyOutputLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}

func TestDuplicateDirectoriesReportOnlyDirectoriesByDefault(t *testing.T) {
	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	aNestedDir := filepath.Join(aDir, "nested")
	bNestedDir := filepath.Join(bDir, "nested")
	excludeFile := filepath.Join(root, "exclude")

	if err := os.MkdirAll(aNestedDir, 0755); err != nil {
		t.Fatalf("create directory a: %v", err)
	}
	if err := os.MkdirAll(bNestedDir, 0755); err != nil {
		t.Fatalf("create directory b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "one.txt"), []byte("same one"), 0644); err != nil {
		t.Fatalf("write a/one.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "one.txt"), []byte("same one"), 0644); err != nil {
		t.Fatalf("write b/one.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "two.txt"), []byte("same two"), 0644); err != nil {
		t.Fatalf("write a/two.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "two.txt"), []byte("same two"), 0644); err != nil {
		t.Fatalf("write b/two.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aNestedDir, "three.txt"), []byte("same three"), 0644); err != nil {
		t.Fatalf("write a/nested/three.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bNestedDir, "three.txt"), []byte("same three"), 0644); err != nil {
		t.Fatalf("write b/nested/three.txt: %v", err)
	}
	if err := os.WriteFile(excludeFile, nil, 0644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	output := runGodedupeHelper(t,
		"-q",
		"-t", aDir,
		"-t", bDir,
		"--exclude-from", excludeFile,
	)

	if strings.Contains(output, "Listing duplicateds for hash") {
		t.Fatalf("output included file duplicate report:\n%s", output)
	}
	if strings.Contains(output, "one.txt") || strings.Contains(output, "two.txt") || strings.Contains(output, "three.txt") {
		t.Fatalf("output included file paths:\n%s", output)
	}
	if strings.Contains(output, aNestedDir) || strings.Contains(output, bNestedDir) {
		t.Fatalf("output included nested duplicate directories:\n%s", output)
	}
	if !strings.Contains(output, "Listing duplicated directories for hash") {
		t.Fatalf("output did not include directory duplicate report:\n%s", output)
	}
	if !outputHasLine(output, aDir) || !outputHasLine(output, bDir) {
		t.Fatalf("output did not include both compared directories:\n%s", output)
	}
}

func TestDuplicateSubdirectoriesAndFilesAreBothReported(t *testing.T) {
	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	aSubDir := filepath.Join(aDir, "sub")
	bSubDir := filepath.Join(bDir, "sub")
	aDupFile := filepath.Join(aDir, "same.txt")
	bDupFile := filepath.Join(bDir, "same.txt")
	excludeFile := filepath.Join(root, "exclude")

	if err := os.MkdirAll(aSubDir, 0755); err != nil {
		t.Fatalf("create subdirectory a: %v", err)
	}
	if err := os.MkdirAll(bSubDir, 0755); err != nil {
		t.Fatalf("create subdirectory b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aSubDir, "inside.txt"), []byte("same inside"), 0644); err != nil {
		t.Fatalf("write a/sub/inside.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bSubDir, "inside.txt"), []byte("same inside"), 0644); err != nil {
		t.Fatalf("write b/sub/inside.txt: %v", err)
	}
	if err := os.WriteFile(aDupFile, []byte("same top-level"), 0644); err != nil {
		t.Fatalf("write a/same.txt: %v", err)
	}
	if err := os.WriteFile(bDupFile, []byte("same top-level"), 0644); err != nil {
		t.Fatalf("write b/same.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "different.txt"), []byte("left"), 0644); err != nil {
		t.Fatalf("write a/different.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "different.txt"), []byte("right"), 0644); err != nil {
		t.Fatalf("write b/different.txt: %v", err)
	}
	if err := os.WriteFile(excludeFile, nil, 0644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	output := runGodedupeHelper(t,
		"-q",
		"-t", aDir,
		"-t", bDir,
		"--exclude-from", excludeFile,
	)

	if !strings.Contains(output, "Listing duplicated directories for hash") {
		t.Fatalf("output did not include directory duplicate report:\n%s", output)
	}
	if !strings.Contains(output, "Listing duplicateds for hash") {
		t.Fatalf("output did not include file duplicate report:\n%s", output)
	}
	if !outputHasLine(output, aSubDir) || !outputHasLine(output, bSubDir) {
		t.Fatalf("output did not include duplicated subdirectories:\n%s", output)
	}
	if outputHasLine(output, aDir) || outputHasLine(output, bDir) {
		t.Fatalf("output included non-duplicated parent directories:\n%s", output)
	}
	if !outputHasLine(output, aDupFile) || !outputHasLine(output, bDupFile) {
		t.Fatalf("output did not include duplicated top-level files:\n%s", output)
	}
	if strings.Contains(output, "inside.txt") {
		t.Fatalf("output included file inside reported duplicate directory:\n%s", output)
	}
	if strings.Contains(output, "different.txt") {
		t.Fatalf("output included non-duplicated file:\n%s", output)
	}
}

func TestSummaryCombinesDuplicatedDirectoriesAndFiles(t *testing.T) {
	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	aSubDir := filepath.Join(aDir, "sub")
	bSubDir := filepath.Join(bDir, "sub")
	excludeFile := filepath.Join(root, "exclude")

	if err := os.MkdirAll(aSubDir, 0755); err != nil {
		t.Fatalf("create subdirectory a: %v", err)
	}
	if err := os.MkdirAll(bSubDir, 0755); err != nil {
		t.Fatalf("create subdirectory b: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aSubDir, "inside.txt"), []byte("same inside"), 0644); err != nil {
		t.Fatalf("write a/sub/inside.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bSubDir, "inside.txt"), []byte("same inside"), 0644); err != nil {
		t.Fatalf("write b/sub/inside.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "same.txt"), []byte("same top-level"), 0644); err != nil {
		t.Fatalf("write a/same.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "same.txt"), []byte("same top-level"), 0644); err != nil {
		t.Fatalf("write b/same.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(aDir, "different.txt"), []byte("left"), 0644); err != nil {
		t.Fatalf("write a/different.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bDir, "different.txt"), []byte("right"), 0644); err != nil {
		t.Fatalf("write b/different.txt: %v", err)
	}
	if err := os.WriteFile(excludeFile, nil, 0644); err != nil {
		t.Fatalf("write exclude file: %v", err)
	}

	output := runGodedupeHelper(t,
		"-q",
		"-m",
		"-t", aDir,
		"-t", bDir,
		"--exclude-from", excludeFile,
	)

	lines := nonEmptyOutputLines(output)
	if len(lines) < 1 {
		t.Fatalf("output had too few lines:\n%s", output)
	}

	want := "1+1=2 duplicated entries in (1+1=2 sets) occupying 11+14=25 bytes"
	if lines[len(lines)-1] != want {
		t.Fatalf("last summary line = %q, want %q\nfull output:\n%s",
			lines[len(lines)-1], want, output)
	}
}

func TestDuplicateDirectoriesOnlyHelper(t *testing.T) {
	if os.Getenv("GODEDUPE_HELPER_PROCESS") != "1" {
		return
	}

	if err := flag.CommandLine.Parse(flag.Args()); err != nil {
		t.Fatalf("parse helper args: %v", err)
	}
	if opt.SameLine {
		opt.quiet = true
	}

	start()
	os.Exit(0)
}
