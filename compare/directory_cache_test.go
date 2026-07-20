package compare

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDirectoryHashCachePathUsesXDGUserDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_USER_DATA_DIR", dataDir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(t.TempDir(), "ignored"))

	path, err := DefaultDirectoryHashCachePath()
	if err != nil {
		t.Fatalf("DefaultDirectoryHashCachePath() returned error: %v", err)
	}

	want := filepath.Join(dataDir, directoryHashCacheFile)
	if path != want {
		t.Fatalf("DefaultDirectoryHashCachePath() = %q, want %q", path, want)
	}
}

func TestDefaultDirectoryHashCachePathUsesXDGDataHome(t *testing.T) {
	dataHome := filepath.Join(t.TempDir(), "share")
	t.Setenv("XDG_USER_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", dataHome)

	path, err := DefaultDirectoryHashCachePath()
	if err != nil {
		t.Fatalf("DefaultDirectoryHashCachePath() returned error: %v", err)
	}

	want := filepath.Join(dataHome, "godedupe", directoryHashCacheFile)
	if path != want {
		t.Fatalf("DefaultDirectoryHashCachePath() = %q, want %q", path, want)
	}
}

func TestEnableDirectoryHashCacheCreatesDatabaseInXDGUserDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	t.Setenv("XDG_USER_DATA_DIR", dataDir)

	if err := EnableDirectoryHashCache(); err != nil {
		t.Fatalf("EnableDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = CloseDirectoryHashCache()
	})

	path := filepath.Join(dataDir, directoryHashCacheFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cache database was not created at %q: %v", path, err)
	}
}

func TestDirectoryHashCacheStoresAndLoadsDirectoryHash(t *testing.T) {
	resetDirectoryCompareState()
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "same content")

	dir := testDirectory(t, root)
	hash, err := computeDirectoryHash(dir, fullHashBlocks)
	if err != nil {
		t.Fatalf("computeDirectoryHash() returned error: %v", err)
	}

	cached, ok := cachedDirectoryHash(testDirectory(t, root), fullHashBlocks)
	if !ok {
		t.Fatal("cachedDirectoryHash() missed stored hash")
	}
	if cached != hash {
		t.Fatalf("cached hash = %x, want %x", cached, hash)
	}
}

func TestDirectoryHashCacheFlushPersistsQueuedDirectoryHash(t *testing.T) {
	resetDirectoryCompareState()
	cachePath := filepath.Join(t.TempDir(), directoryHashCacheFile)
	cache, err := openDirectoryHashCache(cachePath)
	if err != nil {
		t.Fatalf("openDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = cache.Close()
	})
	setTestDirectoryHashCache(t, cache)

	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.txt"), "same content")
	hash, err := computeDirectoryHash(testDirectory(t, root), fullHashBlocks)
	if err != nil {
		t.Fatalf("computeDirectoryHash() returned error: %v", err)
	}
	if err := cache.Flush(); err != nil {
		t.Fatalf("Flush() returned error: %v", err)
	}

	reopened, err := openDirectoryHashCache(cachePath)
	if err != nil {
		t.Fatalf("reopenDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = reopened.Close()
	})
	key, ok := directoryHashCacheKeyFor(testDirectory(t, root), fullHashBlocks)
	if !ok {
		t.Fatal("directoryHashCacheKeyFor() failed")
	}
	cached, ok := reopened.Lookup(key)
	if !ok {
		t.Fatal("reopened cache missed flushed directory hash")
	}
	if cached != hash {
		t.Fatalf("flushed directory hash = %x, want %x", cached, hash)
	}
}

func TestDirectoryHashCacheInvalidatesWhenChildFileMetadataChanges(t *testing.T) {
	resetDirectoryCompareState()
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	root := t.TempDir()
	path := filepath.Join(root, "file.txt")
	writeTestFile(t, path, "same content")

	if _, err := computeDirectoryHash(testDirectory(t, root), fullHashBlocks); err != nil {
		t.Fatalf("computeDirectoryHash() returned error: %v", err)
	}

	writeTestFile(t, path, "new content")
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("change file time: %v", err)
	}

	if _, ok := cachedDirectoryHash(testDirectory(t, root), fullHashBlocks); ok {
		t.Fatal("cachedDirectoryHash() hit after child file metadata changed")
	}
}

func TestCompareDirectoryGroupsUsesCachedDirectoryHashesBeforeHashingFiles(t *testing.T) {
	resetDirectoryCompareState()
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	root := t.TempDir()
	aDir := filepath.Join(root, "a")
	bDir := filepath.Join(root, "b")
	writeTestFile(t, filepath.Join(aDir, "file.txt"), "same content")
	writeTestFile(t, filepath.Join(bDir, "file.txt"), "same content")

	hashA, err := computeDirectoryHash(testDirectory(t, aDir), fullHashBlocks)
	if err != nil {
		t.Fatalf("compute a directory hash: %v", err)
	}
	hashB, err := computeDirectoryHash(testDirectory(t, bDir), fullHashBlocks)
	if err != nil {
		t.Fatalf("compute b directory hash: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("directory hashes differ: %x != %x", hashA, hashB)
	}

	fileHashes = make(map[fileHashKey]uint64)
	dupMap := make(map[uint64]DuplicatedDirectories)
	compareDirectoryGroups(map[uint64]DuplicatedDirectories{
		1: {
			ListDuplicated: []*Directory{
				testDirectory(t, aDir),
				testDirectory(t, bDir),
			},
		},
	}, fullHashBlocks, dupMap, false)

	if len(fileHashes) != 0 {
		t.Fatalf("cached directory compare hashed %d files, want 0", len(fileHashes))
	}
	if got := dupMap[hashA].ListDuplicated; len(got) != 2 {
		t.Fatalf("cached directory duplicate group size = %d, want 2", len(got))
	}
}

func TestLargeFileHashCacheStoresAndLoadsFullFileHash(t *testing.T) {
	resetDirectoryCompareState()
	setLargeFileHashCacheMinSize(t, 1)
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	path := filepath.Join(t.TempDir(), "large.bin")
	writeTestFile(t, path, "cached content")
	file := testFile(t, path)
	want := uint64(0xfeedface)

	storeLargeFileHash(file, fullHashBlocks, want)
	fileHashes = make(map[fileHashKey]uint64)

	got, err := computeFileHash(testFile(t, path), fullHashBlocks)
	if err != nil {
		t.Fatalf("computeFileHash() returned error: %v", err)
	}
	if got != want {
		t.Fatalf("computeFileHash() = %x, want cached hash %x", got, want)
	}
}

func TestDirectoryHashCacheFlushPersistsQueuedLargeFileHash(t *testing.T) {
	resetDirectoryCompareState()
	setLargeFileHashCacheMinSize(t, 1)
	cachePath := filepath.Join(t.TempDir(), directoryHashCacheFile)
	cache, err := openDirectoryHashCache(cachePath)
	if err != nil {
		t.Fatalf("openDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = cache.Close()
	})
	setTestDirectoryHashCache(t, cache)

	path := filepath.Join(t.TempDir(), "large.bin")
	writeTestFile(t, path, "cached content")
	file := testFile(t, path)
	want := uint64(0xfeedface)

	storeLargeFileHash(file, fullHashBlocks, want)
	if err := cache.Flush(); err != nil {
		t.Fatalf("Flush() returned error: %v", err)
	}

	reopened, err := openDirectoryHashCache(cachePath)
	if err != nil {
		t.Fatalf("reopenDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = reopened.Close()
	})
	key, ok := largeFileHashCacheKeyFor(file)
	if !ok {
		t.Fatal("largeFileHashCacheKeyFor() failed")
	}
	cached, ok := reopened.LookupLargeFileHash(key)
	if !ok {
		t.Fatal("reopened cache missed flushed large file hash")
	}
	if cached != want {
		t.Fatalf("flushed large file hash = %x, want %x", cached, want)
	}
}

func TestLargeFileHashCacheSkipsPartialHashes(t *testing.T) {
	resetDirectoryCompareState()
	setLargeFileHashCacheMinSize(t, 1)
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	path := filepath.Join(t.TempDir(), "large.bin")
	writeTestFile(t, path, "cached content")
	file := testFile(t, path)

	storeLargeFileHash(file, partialHashBlocks, 0xfeedface)
	if _, ok := cachedLargeFileHash(file, partialHashBlocks); ok {
		t.Fatal("cachedLargeFileHash() hit for a partial hash")
	}
}

func TestLargeFileHashCacheInvalidatesWhenFileMetadataChanges(t *testing.T) {
	resetDirectoryCompareState()
	setLargeFileHashCacheMinSize(t, 1)
	cache := openTestDirectoryHashCache(t)
	setTestDirectoryHashCache(t, cache)

	path := filepath.Join(t.TempDir(), "large.bin")
	writeTestFile(t, path, "cached content")
	file := testFile(t, path)

	storeLargeFileHash(file, fullHashBlocks, 0xfeedface)
	future := time.Now().Add(time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatalf("change file time: %v", err)
	}

	if _, ok := cachedLargeFileHash(testFile(t, path), fullHashBlocks); ok {
		t.Fatal("cachedLargeFileHash() hit after file metadata changed")
	}
}

func openTestDirectoryHashCache(t *testing.T) *directoryHashCache {
	t.Helper()

	cache, err := openDirectoryHashCache(filepath.Join(t.TempDir(), directoryHashCacheFile))
	if err != nil {
		t.Fatalf("openDirectoryHashCache() returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = cache.Close()
	})

	return cache
}

func setLargeFileHashCacheMinSize(t *testing.T, size int64) {
	t.Helper()

	old := largeFileHashCacheMinSize
	largeFileHashCacheMinSize = size
	t.Cleanup(func() {
		largeFileHashCacheMinSize = old
	})
}

func setTestDirectoryHashCache(t *testing.T, cache *directoryHashCache) {
	t.Helper()

	activeDirectoryHashCacheMu.Lock()
	old := activeDirectoryHashCache
	activeDirectoryHashCache = cache
	activeDirectoryHashCacheMu.Unlock()

	t.Cleanup(func() {
		activeDirectoryHashCacheMu.Lock()
		activeDirectoryHashCache = old
		activeDirectoryHashCacheMu.Unlock()
	})
}
