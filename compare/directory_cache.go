package compare

import (
	"database/sql"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

const directoryHashCacheFile = "directory-hashes.sqlite3"

var (
	activeDirectoryHashCache   *directoryHashCache
	activeDirectoryHashCacheMu sync.RWMutex
	filesystemUIDCache         sync.Map
)

type directoryIdentity struct {
	diskUID string
	inode   string
	mtimeNS int64
	ctimeNS int64
	ok      bool
}

type directoryHashCacheKey struct {
	directoryIdentity
	path         string
	numBlocks    int
	metadataHash uint64
}

type largeFileHashCacheKey struct {
	directoryIdentity
	path string
	size int64
}

type cachedHash struct {
	hash uint64
	ok   bool
}

type directoryHashCache struct {
	db      *sql.DB
	mu      sync.Mutex
	flushMu sync.Mutex

	directoryHashes        map[directoryHashCacheKey]cachedHash
	largeFileHashes        map[largeFileHashCacheKey]cachedHash
	pendingDirectoryHashes map[directoryHashCacheKey]uint64
	pendingLargeFileHashes map[largeFileHashCacheKey]uint64
}

// DefaultDirectoryHashCachePath returns the sqlite database path used for
// persisted directory hashes.
func DefaultDirectoryHashCachePath() (string, error) {
	dir, err := directoryHashCacheDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, directoryHashCacheFile), nil
}

// EnableDirectoryHashCache opens the default persistent directory hash cache.
func EnableDirectoryHashCache() error {
	path, err := DefaultDirectoryHashCachePath()
	if err != nil {
		return err
	}

	cache, err := openDirectoryHashCache(path)
	if err != nil {
		return err
	}

	activeDirectoryHashCacheMu.Lock()
	if activeDirectoryHashCache != nil {
		_ = activeDirectoryHashCache.Close()
	}
	activeDirectoryHashCache = cache
	activeDirectoryHashCacheMu.Unlock()

	return nil
}

// CloseDirectoryHashCache closes the active directory hash cache.
func CloseDirectoryHashCache() error {
	activeDirectoryHashCacheMu.Lock()
	cache := activeDirectoryHashCache
	activeDirectoryHashCache = nil
	activeDirectoryHashCacheMu.Unlock()

	if cache == nil {
		return nil
	}

	return cache.Close()
}

func directoryHashCacheDir() (string, error) {
	if dir := os.Getenv("XDG_USER_DATA_DIR"); dir != "" {
		return expandCacheDir(dir)
	}

	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		expanded, err := expandCacheDir(dir)
		if err != nil {
			return "", err
		}
		return filepath.Join(expanded, "godedupe"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".local", "share", "godedupe"), nil
}

func expandCacheDir(path string) (string, error) {
	if path == "~" {
		return os.UserHomeDir()
	}

	if !strings.HasPrefix(path, "~/") {
		return filepath.Clean(path), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, path[2:]), nil
}

func openDirectoryHashCache(path string) (*directoryHashCache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	cache := &directoryHashCache{
		db:                     db,
		directoryHashes:        make(map[directoryHashCacheKey]cachedHash),
		largeFileHashes:        make(map[largeFileHashCacheKey]cachedHash),
		pendingDirectoryHashes: make(map[directoryHashCacheKey]uint64),
		pendingLargeFileHashes: make(map[largeFileHashCacheKey]uint64),
	}
	if err := cache.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return cache, nil
}

func (cache *directoryHashCache) init() error {
	for _, statement := range []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		`CREATE TABLE IF NOT EXISTS directory_hashes (
			disk_uid TEXT NOT NULL,
			inode TEXT NOT NULL,
			mtime_ns INTEGER NOT NULL,
			num_blocks INTEGER NOT NULL,
			metadata_hash TEXT NOT NULL,
			hash TEXT NOT NULL,
			path TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (disk_uid, inode, mtime_ns, num_blocks, metadata_hash)
		)`,
		`CREATE INDEX IF NOT EXISTS directory_hashes_identity
			ON directory_hashes (disk_uid, inode, num_blocks)`,
		`CREATE TABLE IF NOT EXISTS large_file_hashes (
			disk_uid TEXT NOT NULL,
			inode TEXT NOT NULL,
			mtime_ns INTEGER NOT NULL,
			ctime_ns INTEGER NOT NULL,
			size INTEGER NOT NULL,
			hash TEXT NOT NULL,
			path TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (disk_uid, inode, mtime_ns, ctime_ns, size)
		)`,
		`CREATE INDEX IF NOT EXISTS large_file_hashes_identity
			ON large_file_hashes (disk_uid, inode)`,
	} {
		if _, err := cache.db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}

func (cache *directoryHashCache) Close() error {
	flushErr := cache.Flush()
	closeErr := cache.db.Close()
	if flushErr != nil {
		return flushErr
	}
	return closeErr
}

func (cache *directoryHashCache) Lookup(key directoryHashCacheKey) (uint64, bool) {
	if hash, ok, found := cache.lookupDirectoryHashMemory(key); found {
		return hash, ok
	}

	var hashText string
	err := cache.db.QueryRow(
		`SELECT hash
		 FROM directory_hashes
		 WHERE disk_uid = ?
		   AND inode = ?
		   AND mtime_ns = ?
		   AND num_blocks = ?
		   AND metadata_hash = ?`,
		key.diskUID,
		key.inode,
		key.mtimeNS,
		key.numBlocks,
		strconv.FormatUint(key.metadataHash, 10),
	).Scan(&hashText)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		cache.rememberDirectoryHash(key, 0, false)
		return 0, false
	}

	hash, err := strconv.ParseUint(hashText, 10, 64)
	if err != nil {
		cache.rememberDirectoryHash(key, 0, false)
		return 0, false
	}

	cache.rememberDirectoryHash(key, hash, true)
	return hash, true
}

func (cache *directoryHashCache) Store(key directoryHashCacheKey, hash uint64) error {
	cache.mu.Lock()
	cache.directoryHashes[key] = cachedHash{hash: hash, ok: true}
	cache.pendingDirectoryHashes[key] = hash
	cache.mu.Unlock()
	return nil
}

func (cache *directoryHashCache) LookupLargeFileHash(key largeFileHashCacheKey) (uint64, bool) {
	if hash, ok, found := cache.lookupLargeFileHashMemory(key); found {
		return hash, ok
	}

	var hashText string
	err := cache.db.QueryRow(
		`SELECT hash
		 FROM large_file_hashes
		 WHERE disk_uid = ?
		   AND inode = ?
		   AND mtime_ns = ?
		   AND ctime_ns = ?
		   AND size = ?`,
		key.diskUID,
		key.inode,
		key.mtimeNS,
		key.ctimeNS,
		key.size,
	).Scan(&hashText)
	if errors.Is(err, sql.ErrNoRows) || err != nil {
		cache.rememberLargeFileHash(key, 0, false)
		return 0, false
	}

	hash, err := strconv.ParseUint(hashText, 10, 64)
	if err != nil {
		cache.rememberLargeFileHash(key, 0, false)
		return 0, false
	}

	cache.rememberLargeFileHash(key, hash, true)
	return hash, true
}

func (cache *directoryHashCache) StoreLargeFileHash(key largeFileHashCacheKey, hash uint64) error {
	cache.mu.Lock()
	cache.largeFileHashes[key] = cachedHash{hash: hash, ok: true}
	cache.pendingLargeFileHashes[key] = hash
	cache.mu.Unlock()
	return nil
}

func (cache *directoryHashCache) Flush() error {
	cache.flushMu.Lock()
	defer cache.flushMu.Unlock()

	directoryHashes, largeFileHashes := cache.pendingHashSnapshot()
	if len(directoryHashes) == 0 && len(largeFileHashes) == 0 {
		return nil
	}

	tx, err := cache.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := flushDirectoryHashStores(tx, directoryHashes); err != nil {
		return err
	}
	if err := flushLargeFileHashStores(tx, largeFileHashes); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	cache.clearPendingHashSnapshot(directoryHashes, largeFileHashes)
	return nil
}

func (cache *directoryHashCache) pendingHashSnapshot() (map[directoryHashCacheKey]uint64, map[largeFileHashCacheKey]uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	directoryHashes := make(map[directoryHashCacheKey]uint64, len(cache.pendingDirectoryHashes))
	for key, hash := range cache.pendingDirectoryHashes {
		directoryHashes[key] = hash
	}

	largeFileHashes := make(map[largeFileHashCacheKey]uint64, len(cache.pendingLargeFileHashes))
	for key, hash := range cache.pendingLargeFileHashes {
		largeFileHashes[key] = hash
	}

	return directoryHashes, largeFileHashes
}

func (cache *directoryHashCache) clearPendingHashSnapshot(directoryHashes map[directoryHashCacheKey]uint64, largeFileHashes map[largeFileHashCacheKey]uint64) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for key, hash := range directoryHashes {
		if cache.pendingDirectoryHashes[key] == hash {
			delete(cache.pendingDirectoryHashes, key)
		}
	}
	for key, hash := range largeFileHashes {
		if cache.pendingLargeFileHashes[key] == hash {
			delete(cache.pendingLargeFileHashes, key)
		}
	}
}

func flushDirectoryHashStores(tx *sql.Tx, hashes map[directoryHashCacheKey]uint64) error {
	for key, hash := range hashes {
		metadataHash := strconv.FormatUint(key.metadataHash, 10)
		if _, err := tx.Exec(
			`DELETE FROM directory_hashes
			 WHERE disk_uid = ?
			   AND inode = ?
			   AND num_blocks = ?
			   AND NOT (mtime_ns = ? AND metadata_hash = ?)`,
			key.diskUID,
			key.inode,
			key.numBlocks,
			key.mtimeNS,
			metadataHash,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO directory_hashes (
				disk_uid, inode, mtime_ns, num_blocks, metadata_hash, hash, path, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (disk_uid, inode, mtime_ns, num_blocks, metadata_hash)
			DO UPDATE SET
				hash = excluded.hash,
				path = excluded.path,
				updated_at = excluded.updated_at`,
			key.diskUID,
			key.inode,
			key.mtimeNS,
			key.numBlocks,
			metadataHash,
			strconv.FormatUint(hash, 10),
			key.path,
			time.Now().Unix(),
		); err != nil {
			return err
		}
	}

	return nil
}

func flushLargeFileHashStores(tx *sql.Tx, hashes map[largeFileHashCacheKey]uint64) error {
	for key, hash := range hashes {
		if _, err := tx.Exec(
			`DELETE FROM large_file_hashes
			 WHERE disk_uid = ?
			   AND inode = ?
			   AND NOT (mtime_ns = ? AND ctime_ns = ? AND size = ?)`,
			key.diskUID,
			key.inode,
			key.mtimeNS,
			key.ctimeNS,
			key.size,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO large_file_hashes (
				disk_uid, inode, mtime_ns, ctime_ns, size, hash, path, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (disk_uid, inode, mtime_ns, ctime_ns, size)
			DO UPDATE SET
				hash = excluded.hash,
				path = excluded.path,
				updated_at = excluded.updated_at`,
			key.diskUID,
			key.inode,
			key.mtimeNS,
			key.ctimeNS,
			key.size,
			strconv.FormatUint(hash, 10),
			key.path,
			time.Now().Unix(),
		); err != nil {
			return err
		}
	}

	return nil
}

func (cache *directoryHashCache) lookupDirectoryHashMemory(key directoryHashCacheKey) (uint64, bool, bool) {
	cache.mu.Lock()
	hash, found := cache.directoryHashes[key]
	cache.mu.Unlock()
	return hash.hash, hash.ok, found
}

func (cache *directoryHashCache) rememberDirectoryHash(key directoryHashCacheKey, hash uint64, ok bool) {
	cache.mu.Lock()
	cache.directoryHashes[key] = cachedHash{hash: hash, ok: ok}
	cache.mu.Unlock()
}

func (cache *directoryHashCache) lookupLargeFileHashMemory(key largeFileHashCacheKey) (uint64, bool, bool) {
	cache.mu.Lock()
	hash, found := cache.largeFileHashes[key]
	cache.mu.Unlock()
	return hash.hash, hash.ok, found
}

func (cache *directoryHashCache) rememberLargeFileHash(key largeFileHashCacheKey, hash uint64, ok bool) {
	cache.mu.Lock()
	cache.largeFileHashes[key] = cachedHash{hash: hash, ok: ok}
	cache.mu.Unlock()
}

func cachedDirectoryHash(dir *Directory, numBlocks int) (uint64, bool) {
	cache := currentDirectoryHashCache()
	if cache == nil {
		return 0, false
	}

	key, ok := directoryHashCacheKeyFor(dir, numBlocks)
	if !ok {
		return 0, false
	}

	return cache.Lookup(key)
}

func cachedDirectoryContentHash(dir *Directory, numBlocks int) (uint64, bool) {
	if dir == nil {
		return 0, false
	}

	finalizeDirectory(dir)
	if dir.contentHashes != nil {
		if contentHash, ok := dir.contentHashes[numBlocks]; ok {
			return contentHash, true
		}
	}

	contentHash, ok := cachedDirectoryHash(dir, numBlocks)
	if !ok {
		return 0, false
	}

	if dir.contentHashes == nil {
		dir.contentHashes = make(map[int]uint64)
	}
	dir.contentHashes[numBlocks] = contentHash

	return contentHash, true
}

func cachedLargeFileHash(file File, numBlocks int) (uint64, bool) {
	if !shouldPersistFileHash(file, numBlocks) {
		return 0, false
	}

	cache := currentDirectoryHashCache()
	if cache == nil {
		return 0, false
	}

	key, ok := largeFileHashCacheKeyFor(file)
	if !ok {
		return 0, false
	}

	return cache.LookupLargeFileHash(key)
}

func storeDirectoryHash(dir *Directory, numBlocks int, hash uint64) {
	cache := currentDirectoryHashCache()
	if cache == nil {
		return
	}

	key, ok := directoryHashCacheKeyFor(dir, numBlocks)
	if !ok {
		return
	}

	_ = cache.Store(key, hash)
}

func storeLargeFileHash(file File, numBlocks int, hash uint64) {
	if !shouldPersistFileHash(file, numBlocks) {
		return
	}

	cache := currentDirectoryHashCache()
	if cache == nil {
		return
	}

	key, ok := largeFileHashCacheKeyFor(file)
	if !ok {
		return
	}

	_ = cache.StoreLargeFileHash(key, hash)
}

func currentDirectoryHashCache() *directoryHashCache {
	activeDirectoryHashCacheMu.RLock()
	cache := activeDirectoryHashCache
	activeDirectoryHashCacheMu.RUnlock()
	return cache
}

func shouldPersistFileHash(file File, numBlocks int) bool {
	return numBlocks == fullHashBlocks &&
		file.Info != nil &&
		file.Info.Size() >= largeFileHashCacheMinSize
}

func directoryHashCacheKeyFor(dir *Directory, numBlocks int) (directoryHashCacheKey, bool) {
	if dir == nil {
		return directoryHashCacheKey{}, false
	}

	finalizeDirectory(dir)

	identity := dir.cacheIdentity
	if !identity.ok {
		identity = newDirectoryIdentity(dir.Path, dir.Info)
	}
	if !identity.ok {
		return directoryHashCacheKey{}, false
	}

	return directoryHashCacheKey{
		directoryIdentity: identity,
		path:              filepath.Clean(dir.Path),
		numBlocks:         numBlocks,
		metadataHash:      dir.metadataHash,
	}, true
}

func largeFileHashCacheKeyFor(file File) (largeFileHashCacheKey, bool) {
	identity := newPathIdentity(file.Path, file.Info)
	if !identity.ok {
		return largeFileHashCacheKey{}, false
	}

	return largeFileHashCacheKey{
		directoryIdentity: identity,
		path:              filepath.Clean(file.Path),
		size:              file.Info.Size(),
	}, true
}

func newDirectoryIdentity(path string, info os.FileInfo) directoryIdentity {
	return newPathIdentity(path, info)
}

func newPathIdentity(path string, info os.FileInfo) directoryIdentity {
	if info == nil && path != "" {
		var err error
		info, err = os.Stat(path)
		if err != nil {
			return directoryIdentity{}
		}
	}

	identity := newFileIdentity(info)
	if !identity.ok {
		return identity
	}

	if diskUID, ok := cachedFilesystemUID(path, identity.diskUID); ok {
		identity.diskUID = diskUID
	}

	return identity
}

func newFileIdentity(info os.FileInfo) directoryIdentity {
	if info == nil {
		return directoryIdentity{}
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return directoryIdentity{mtimeNS: info.ModTime().UnixNano()}
	}

	return directoryIdentity{
		diskUID: strconv.FormatUint(stat.Dev, 10),
		inode:   strconv.FormatUint(stat.Ino, 10),
		mtimeNS: stat.Mtim.Sec*1e9 + stat.Mtim.Nsec,
		ctimeNS: stat.Ctim.Sec*1e9 + stat.Ctim.Nsec,
		ok:      true,
	}
}

func filesystemUID(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return "", false
	}

	return fmt.Sprintf("%08x%08x", uint32(stat.Fsid.X__val[0]), uint32(stat.Fsid.X__val[1])), true
}

func cachedFilesystemUID(path string, deviceUID string) (string, bool) {
	if deviceUID != "" {
		if diskUID, ok := filesystemUIDCache.Load(deviceUID); ok {
			return diskUID.(string), true
		}
	}

	diskUID, ok := filesystemUID(path)
	if ok && deviceUID != "" {
		filesystemUIDCache.Store(deviceUID, diskUID)
	}

	return diskUID, ok
}

func writeEntryMetadata(hash hash.Hash64, fallback string, info os.FileInfo, identity directoryIdentity) {
	if identity.ok {
		writeHashString(hash, identity.diskUID)
		writeHashString(hash, identity.inode)
		writeHashInt64(hash, identity.mtimeNS)
		writeHashInt64(hash, identity.ctimeNS)
		return
	}

	writeHashString(hash, fallback)
	if info != nil {
		writeHashInt64(hash, info.ModTime().UnixNano())
		writeHashUint64(hash, uint64(info.Mode()))
	}
}
