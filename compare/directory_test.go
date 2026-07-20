package compare

import (
	"os"
	"path/filepath"
	"testing"
)

func resetDirectoryCompareState() {
	dupFileSize = make(map[int64]Duplicated)
	partialDuplicatedFiles = make(map[uint64]Duplicated)
	DuplicatedFiles = make(map[uint64]Duplicated)
	dupDirFileCount = make(map[int64]DuplicatedDirectories)
	structureDuplicatedDirectories = make(map[uint64]DuplicatedDirectories)
	partialDuplicatedDirectories = make(map[uint64]DuplicatedDirectories)
	DuplicatedDirectoriesFound = make(map[uint64]DuplicatedDirectories)
	fileHashes = make(map[fileHashKey]uint64)
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create parent directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
}

func testDirectory(t *testing.T, path string) *Directory {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat test directory: %v", err)
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read test directory: %v", err)
	}

	dir := &Directory{Path: path, Info: info}
	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			dir.Children = append(dir.Children, testDirectory(t, entryPath))
			continue
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat test file: %v", err)
		}
		dir.Files = append(dir.Files, File{Path: entryPath, Info: info})
	}

	return dir
}

func testFile(t *testing.T, path string) File {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat test file: %v", err)
	}

	return File{Path: path, Info: info}
}

func TestValidateDuplicatedDirectories(t *testing.T) {
	resetDirectoryCompareState()

	root := t.TempDir()
	sharedPrefix := string(make([]byte, bufferSize))
	sharedContent := sharedPrefix + "same"
	differentContent := sharedPrefix + "diff"

	writeTestFile(t, filepath.Join(root, "a", "sub", "data.bin"), sharedContent)
	writeTestFile(t, filepath.Join(root, "a", "note.txt"), "same note")
	writeTestFile(t, filepath.Join(root, "b", "sub", "data.bin"), sharedContent)
	writeTestFile(t, filepath.Join(root, "b", "note.txt"), "same note")
	writeTestFile(t, filepath.Join(root, "c", "sub", "data.bin"), differentContent)
	writeTestFile(t, filepath.Join(root, "c", "note.txt"), "same note")

	AddDirectory(testDirectory(t, filepath.Join(root, "a")))
	AddDirectory(testDirectory(t, filepath.Join(root, "b")))
	AddDirectory(testDirectory(t, filepath.Join(root, "c")))

	ValidateDuplicatedDirectories(false)

	if len(DuplicatedDirectoriesFound) != 1 {
		t.Fatalf("found %d duplicated directory sets, want 1", len(DuplicatedDirectoriesFound))
	}

	for _, duplicated := range DuplicatedDirectoriesFound {
		if len(duplicated.ListDuplicated) != 2 {
			t.Fatalf("duplicated directories = %d, want 2", len(duplicated.ListDuplicated))
		}

		paths := map[string]bool{}
		for _, dir := range duplicated.ListDuplicated {
			paths[dir.Path] = true
		}

		if !paths[filepath.Join(root, "a")] || !paths[filepath.Join(root, "b")] {
			t.Fatalf("duplicated directories = %#v, want a and b", paths)
		}
	}
}

func TestFilterFilesCoveredByDirectories(t *testing.T) {
	root := t.TempDir()

	coveredA := filepath.Join(root, "a", "sub", "inside.txt")
	coveredB := filepath.Join(root, "b", "sub", "inside.txt")
	uncoveredA := filepath.Join(root, "a", "same.txt")
	uncoveredB := filepath.Join(root, "b", "same.txt")
	mixed := filepath.Join(root, "c", "inside.txt")

	writeTestFile(t, coveredA, "covered")
	writeTestFile(t, coveredB, "covered")
	writeTestFile(t, uncoveredA, "uncovered")
	writeTestFile(t, uncoveredB, "uncovered")
	writeTestFile(t, mixed, "covered")

	dupFiles := map[uint64]Duplicated{
		1: {ListDuplicated: []File{testFile(t, coveredA), testFile(t, coveredB)}},
		2: {ListDuplicated: []File{testFile(t, uncoveredA), testFile(t, uncoveredB)}},
		3: {ListDuplicated: []File{testFile(t, coveredA), testFile(t, mixed)}},
	}
	dupDirs := map[uint64]DuplicatedDirectories{
		4: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a", "sub")},
				{Path: filepath.Join(root, "b", "sub")},
			},
		},
	}

	filtered := FilterFilesCoveredByDirectories(dupFiles, dupDirs, false)

	if len(filtered) != 1 {
		t.Fatalf("filtered duplicated files = %d sets, want 1", len(filtered))
	}

	files, ok := filtered[2]
	if !ok {
		t.Fatalf("filtered duplicated files did not keep the uncovered set")
	}
	if len(files.ListDuplicated) != 2 {
		t.Fatalf("uncovered file set size = %d, want 2", len(files.ListDuplicated))
	}
}

func TestFilterFilesCoveredByDirectoriesKeepsFilesWithoutDuplicatedDirectories(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	second := filepath.Join(root, "second.txt")

	writeTestFile(t, first, "same")
	writeTestFile(t, second, "same")

	dupFiles := map[uint64]Duplicated{
		1: {ListDuplicated: []File{testFile(t, first), testFile(t, second)}},
	}

	filtered := FilterFilesCoveredByDirectories(dupFiles, nil, false)

	if len(filtered) != 1 {
		t.Fatalf("filtered duplicated files = %d sets, want 1", len(filtered))
	}
}

func TestPruneFileCandidatesCoveredByDirectories(t *testing.T) {
	resetDirectoryCompareState()
	root := t.TempDir()

	coveredA := filepath.Join(root, "a", "sub", "covered-a.txt")
	coveredB := filepath.Join(root, "b", "sub", "covered-b.txt")
	uncoveredA := filepath.Join(root, "a", "same.txt")
	uncoveredB := filepath.Join(root, "b", "same.txt")

	writeTestFile(t, coveredA, "same-size")
	writeTestFile(t, coveredB, "same-size")
	writeTestFile(t, uncoveredA, "same-size")
	writeTestFile(t, uncoveredB, "same-size")

	dupFileSize = map[int64]Duplicated{
		1: {
			ListDuplicated: []File{
				testFile(t, coveredA),
				testFile(t, coveredB),
				testFile(t, uncoveredA),
				testFile(t, uncoveredB),
			},
		},
		2: {
			ListDuplicated: []File{
				testFile(t, coveredA),
				testFile(t, coveredB),
			},
		},
	}

	PruneFileCandidatesCoveredByDirectories(map[uint64]DuplicatedDirectories{
		3: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a", "sub")},
				{Path: filepath.Join(root, "b", "sub")},
			},
		},
	})

	files := dupFileSize[1].ListDuplicated
	if len(files) != 2 {
		t.Fatalf("remaining candidates = %d, want 2", len(files))
	}
	remaining := map[string]bool{}
	for _, file := range files {
		remaining[file.Path] = true
	}
	if !remaining[uncoveredA] || !remaining[uncoveredB] {
		t.Fatalf("remaining candidates = %#v, want uncovered files", remaining)
	}
	if _, ok := dupFileSize[2]; ok {
		t.Fatal("covered-only candidate group was not removed")
	}
}

func TestDuplicatedDirectoryIndexContainsOnlyDescendants(t *testing.T) {
	root := t.TempDir()
	index := newDuplicatedDirectoryIndex(map[uint64]DuplicatedDirectories{
		1: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a")},
			},
		},
	})

	tests := []struct {
		path string
		want bool
	}{
		{path: filepath.Join(root, "a", "file.txt"), want: true},
		{path: filepath.Join(root, "a", "sub", "file.txt"), want: true},
		{path: filepath.Join(root, "a"), want: false},
		{path: filepath.Join(root, "a2", "file.txt"), want: false},
		{path: filepath.Join(root, "ab", "file.txt"), want: false},
		{path: root, want: false},
	}

	for _, tt := range tests {
		if got := index.containsParentOf(tt.path); got != tt.want {
			t.Fatalf("containsParentOf(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestRemoveNestedDuplicatedDirectoriesUsesPathIndex(t *testing.T) {
	resetDirectoryCompareState()
	root := t.TempDir()
	parentHash := uint64(1)
	childHash := uint64(2)

	DuplicatedDirectoriesFound = map[uint64]DuplicatedDirectories{
		parentHash: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a")},
				{Path: filepath.Join(root, "b")},
			},
		},
		childHash: {
			ListDuplicated: []*Directory{
				{Path: filepath.Join(root, "a", "sub")},
				{Path: filepath.Join(root, "b", "sub")},
			},
		},
	}

	removeNestedDuplicatedDirectories()

	if _, ok := DuplicatedDirectoriesFound[parentHash]; !ok {
		t.Fatal("parent directory duplicate set was removed")
	}
	if _, ok := DuplicatedDirectoriesFound[childHash]; ok {
		t.Fatal("nested directory duplicate set was kept")
	}
}

func TestDirectoryHashIncludesRelativePaths(t *testing.T) {
	resetDirectoryCompareState()

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a", "first.txt"), "same content")
	writeTestFile(t, filepath.Join(root, "b", "second.txt"), "same content")

	AddDirectory(testDirectory(t, filepath.Join(root, "a")))
	AddDirectory(testDirectory(t, filepath.Join(root, "b")))

	ValidateDuplicatedDirectories(false)

	if len(DuplicatedDirectoriesFound) != 0 {
		t.Fatalf("found %d duplicated directory sets, want 0", len(DuplicatedDirectoriesFound))
	}
}

func TestDirectoryHashIncludesEmptyChildDirectories(t *testing.T) {
	resetDirectoryCompareState()

	root := t.TempDir()
	left := filepath.Join(root, "a", "left")
	right := filepath.Join(root, "b", "right")
	if err := os.MkdirAll(left, 0755); err != nil {
		t.Fatalf("create left child: %v", err)
	}
	if err := os.MkdirAll(right, 0755); err != nil {
		t.Fatalf("create right child: %v", err)
	}

	hashA, err := computeDirectoryHash(testDirectory(t, filepath.Join(root, "a")), fullHashBlocks)
	if err != nil {
		t.Fatalf("compute a directory hash: %v", err)
	}
	hashB, err := computeDirectoryHash(testDirectory(t, filepath.Join(root, "b")), fullHashBlocks)
	if err != nil {
		t.Fatalf("compute b directory hash: %v", err)
	}

	if hashA == hashB {
		t.Fatalf("directory hashes matched for different empty child trees: %x", hashA)
	}
}

func TestValidateDuplicatedDirectoriesIgnoresTreesWithoutComparableFiles(t *testing.T) {
	resetDirectoryCompareState()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a", "sub"), 0755); err != nil {
		t.Fatalf("create a/sub: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "b", "sub"), 0755); err != nil {
		t.Fatalf("create b/sub: %v", err)
	}

	AddDirectory(testDirectory(t, filepath.Join(root, "a")))
	AddDirectory(testDirectory(t, filepath.Join(root, "b")))
	ValidateDuplicatedDirectories(false)

	if len(DuplicatedDirectoriesFound) != 0 {
		t.Fatalf("found %d duplicated empty directory-tree sets, want 0", len(DuplicatedDirectoriesFound))
	}
}
