package compare

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = w

	out := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		out <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout pipe writer: %v", err)
	}
	os.Stdout = oldStdout

	return <-out
}

func TestFileHashGroupsReportSetProgressWhenVerbose(t *testing.T) {
	resetDirectoryCompareState()

	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "a1.bin"),
		filepath.Join(root, "a2.bin"),
		filepath.Join(root, "b1.bin"),
		filepath.Join(root, "b2.bin"),
	}
	for _, path := range paths {
		writeTestFile(t, path, strings.Repeat("x", bufferSize*2))
	}

	groups := []fileHashGroup{
		{files: []File{testFile(t, paths[0]), testFile(t, paths[1])}, numBlocks: fullHashBlocks},
		{files: []File{testFile(t, paths[2]), testFile(t, paths[3])}, numBlocks: fullHashBlocks},
	}

	output := captureStdout(t, func() {
		compareFileGroups(groups, make(map[uint64]Duplicated), true)
	})

	if !strings.Contains(output, "[+] 1 / 2 done") || !strings.Contains(output, "[+] 2 / 2 done") {
		t.Fatalf("hash progress did not report set completion:\n%s", output)
	}
	for _, path := range paths {
		if strings.Contains(output, filepath.Base(path)) || strings.Contains(output, path) {
			t.Fatalf("hash progress reported a file path:\n%s", output)
		}
	}
	if strings.Contains(output, "Hashing files") || strings.Contains(output, "1 / 4 done") {
		t.Fatalf("hash progress reported file counts:\n%s", output)
	}
	if strings.Contains(output, "bytes read") {
		t.Fatalf("hash progress reported in-file bytes:\n%s", output)
	}
}

func TestFileHashGroupsDoNotReportProgressWhenQuiet(t *testing.T) {
	resetDirectoryCompareState()

	path := filepath.Join(t.TempDir(), "large.bin")
	writeTestFile(t, path, strings.Repeat("x", bufferSize*2))

	output := captureStdout(t, func() {
		compareFileGroups([]fileHashGroup{
			{files: []File{testFile(t, path)}, numBlocks: fullHashBlocks},
		}, make(map[uint64]Duplicated), false)
	})

	if output != "" {
		t.Fatalf("hash progress reported in quiet mode:\n%s", output)
	}
}

func TestEmptyFileStagesDoNotPrintBlankCompletionLines(t *testing.T) {
	resetDirectoryCompareState()

	output := captureStdout(t, func() {
		doPartialFileCompare(true)
		doFullFileCompare(true)
	})

	if strings.Contains(output, "Starting stage 2 / 3.\n\n[+] Stage 2 / 3 completed.") {
		t.Fatalf("empty partial file stage printed a blank completion line:\n%s", output)
	}
	if strings.Contains(output, "Starting stage 3 / 3.\n\n[+] Stage 3 / 3 completed.") {
		t.Fatalf("empty full file stage printed a blank completion line:\n%s", output)
	}
}

func TestEmptyDirectoryStagesDoNotPrintBlankCompletionLines(t *testing.T) {
	resetDirectoryCompareState()

	output := captureStdout(t, func() {
		doPartialDirectoryCompare(true)
		doFullDirectoryCompare(true)
	})

	if strings.Contains(output, "Starting directory stage 2 / 3.\n\n[+] Directory stage 2 / 3 completed.") {
		t.Fatalf("empty partial directory stage printed a blank completion line:\n%s", output)
	}
	if strings.Contains(output, "Starting directory stage 3 / 3.\n\n[+] Directory stage 3 / 3 completed.") {
		t.Fatalf("empty full directory stage printed a blank completion line:\n%s", output)
	}
}

func TestFilterFilesCoveredByDirectoriesDoesNotReportPostProcessingProgress(t *testing.T) {
	root := t.TempDir()
	coveredA := filepath.Join(root, "a", "sub", "inside.txt")
	coveredB := filepath.Join(root, "b", "sub", "inside.txt")
	uncoveredA := filepath.Join(root, "a", "same.txt")
	uncoveredB := filepath.Join(root, "b", "same.txt")

	writeTestFile(t, coveredA, "covered")
	writeTestFile(t, coveredB, "covered")
	writeTestFile(t, uncoveredA, "uncovered")
	writeTestFile(t, uncoveredB, "uncovered")

	dupFiles := map[uint64]Duplicated{
		1: {ListDuplicated: []File{testFile(t, coveredA), testFile(t, coveredB)}},
		2: {ListDuplicated: []File{testFile(t, uncoveredA), testFile(t, uncoveredB)}},
	}
	dupDirs := map[uint64]DuplicatedDirectories{
		3: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a", "sub")},
				{Path: filepath.Join(root, "b", "sub")},
			},
		},
	}

	output := captureStdout(t, func() {
		FilterFilesCoveredByDirectories(dupFiles, dupDirs, true)
	})

	if output != "" {
		t.Fatalf("post-processing reported progress:\n%s", output)
	}
}

func TestRemoveNestedDuplicatedDirectoriesDoesNotReportPostProcessingProgress(t *testing.T) {
	resetDirectoryCompareState()
	root := t.TempDir()

	DuplicatedDirectoriesFound = map[uint64]DuplicatedDirectories{
		1: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a")},
				{Path: filepath.Join(root, "b")},
			},
		},
		2: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a", "sub")},
				{Path: filepath.Join(root, "b", "sub")},
			},
		},
	}

	output := captureStdout(t, func() {
		removeNestedDuplicatedDirectories()
	})

	if output != "" {
		t.Fatalf("directory post-processing reported progress:\n%s", output)
	}
}
