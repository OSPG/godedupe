package compare

import (
	"os"
	"path/filepath"
	"testing"
)

func resetDirectoryCompareState() {
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

	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatalf("read test directory: %v", err)
	}

	dir := &Directory{Path: path}
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

	filtered := FilterFilesCoveredByDirectories(dupFiles, dupDirs)

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
