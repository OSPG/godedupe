package compare

import (
	"encoding/binary"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
)

// Directory contains a comparable directory tree.
type Directory struct {
	Path     string
	Info     os.FileInfo
	Files    []File
	Children []*Directory
	// Incomplete means the tree has child directories that were not scanned.
	Incomplete bool

	fileCount     int64
	size          int64
	structureHash uint64
	metadataHash  uint64
	cacheIdentity directoryIdentity
	contentHashes map[int]uint64
	finalized     bool
}

// DuplicatedDirectories stores directories with matching comparison keys.
type DuplicatedDirectories = duplicateGroup[*Directory]

var (
	dupDirFileCount                = make(map[int64]DuplicatedDirectories)
	structureDuplicatedDirectories = make(map[uint64]DuplicatedDirectories)
	partialDuplicatedDirectories   = make(map[uint64]DuplicatedDirectories)
	// DuplicatedDirectoriesFound stores all known duplicated directories.
	DuplicatedDirectoriesFound = make(map[uint64]DuplicatedDirectories)
)

var (
	fileEntryMarker      = []byte("F")
	directoryEntryMarker = []byte("D")
)

// Size returns the sum of all comparable files in the directory.
func (dir *Directory) Size() int64 {
	if dir == nil {
		return 0
	}
	finalizeDirectory(dir)
	return dir.size
}

// AddDirectory appends directories to the file-count map to be compared later.
func AddDirectory(dir *Directory) {
	if dir == nil {
		return
	}

	finalizeDirectory(dir)
	if dir.Incomplete || dir.fileCount == 0 {
		return
	}
	addToGroup(dupDirFileCount, dir.fileCount, dir)
}

func finalizeDirectory(dir *Directory) {
	if dir.finalized {
		return
	}

	sort.Slice(dir.Files, func(i, j int) bool {
		return dir.Files[i].Info.Name() < dir.Files[j].Info.Name()
	})
	sort.Slice(dir.Children, func(i, j int) bool {
		return filepath.Base(dir.Children[i].Path) < filepath.Base(dir.Children[j].Path)
	})

	structureHash := crcPool.Get().(hash.Hash64)
	metadataHash := crcPool.Get().(hash.Hash64)
	defer func() {
		structureHash.Reset()
		crcPool.Put(structureHash)
		metadataHash.Reset()
		crcPool.Put(metadataHash)
	}()

	dir.cacheIdentity = newDirectoryIdentity(dir.Path, dir.Info)
	writeEntryMetadata(metadataHash, "S", dir.Info, dir.cacheIdentity)

	for _, file := range dir.Files {
		dir.fileCount++
		dir.size += file.Info.Size()
		structureHash.Write(fileEntryMarker)
		writeHashString(structureHash, file.Info.Name())
		writeHashUint64(structureHash, uint64(file.Info.Size()))

		metadataHash.Write(fileEntryMarker)
		writeHashString(metadataHash, file.Info.Name())
		writeHashUint64(metadataHash, uint64(file.Info.Size()))
		writeEntryMetadata(metadataHash, "", file.Info, newFileIdentity(file.Info))
	}

	for _, child := range dir.Children {
		finalizeDirectory(child)
		dir.fileCount += child.fileCount
		dir.size += child.size
		structureHash.Write(directoryEntryMarker)
		writeHashString(structureHash, filepath.Base(child.Path))
		writeHashUint64(structureHash, uint64(child.fileCount))
		writeHashUint64(structureHash, child.structureHash)

		metadataHash.Write(directoryEntryMarker)
		writeHashString(metadataHash, filepath.Base(child.Path))
		writeHashUint64(metadataHash, uint64(child.fileCount))
		writeHashUint64(metadataHash, child.structureHash)
		writeHashUint64(metadataHash, child.metadataHash)
	}

	dir.structureHash = structureHash.Sum64()
	dir.metadataHash = metadataHash.Sum64()
	dir.finalized = true
}

// ValidateDuplicatedDirectories does the full hash of duplicated directories to
// avoid false positives.
func ValidateDuplicatedDirectories(verbose bool) {
	doPartialDirectoryCompare(verbose)
	doFullDirectoryCompare(verbose)
}

// FilterFilesCoveredByDirectories removes duplicated file entries that are
// already represented by reported duplicated directories.
func FilterFilesCoveredByDirectories(dupFiles map[uint64]Duplicated, dupDirs map[uint64]DuplicatedDirectories, _ bool) map[uint64]Duplicated {
	filtered := make(map[uint64]Duplicated)
	if len(dupFiles) == 0 {
		return filtered
	}
	if len(dupDirs) == 0 {
		return dupFiles
	}

	coveredDirectories := newDuplicatedDirectoryIndex(dupDirs)
	if len(coveredDirectories) == 0 {
		return dupFiles
	}

	for fileHash, duplicated := range dupFiles {
		var files []File
		for _, file := range duplicated.ListDuplicated {
			if !coveredDirectories.containsParentOf(file.Path) {
				files = append(files, file)
			}
		}

		if len(files) > 1 {
			filtered[fileHash] = Duplicated{ListDuplicated: files}
		}
	}

	return filtered
}

// PruneFileCandidatesCoveredByDirectories removes file candidates that are
// already covered by duplicated directories before file hashing starts.
func PruneFileCandidatesCoveredByDirectories(dupDirs map[uint64]DuplicatedDirectories) {
	if len(dupFileSize) == 0 || len(dupDirs) == 0 {
		return
	}

	coveredDirectories := newDuplicatedDirectoryIndex(dupDirs)
	if len(coveredDirectories) == 0 {
		return
	}

	for size, duplicated := range dupFileSize {
		files := duplicated.ListDuplicated[:0]
		for _, file := range duplicated.ListDuplicated {
			if !coveredDirectories.containsParentOf(file.Path) {
				files = append(files, file)
			}
		}

		if len(files) > 1 {
			dupFileSize[size] = Duplicated{ListDuplicated: files}
		} else {
			delete(dupFileSize, size)
		}
	}
}

type duplicatedDirectoryIndex map[string]struct{}

func newDuplicatedDirectoryIndex(dupDirs map[uint64]DuplicatedDirectories) duplicatedDirectoryIndex {
	index := make(duplicatedDirectoryIndex)
	for _, duplicated := range dupDirs {
		for _, dir := range duplicated.ListDuplicated {
			if dir == nil || dir.Path == "" {
				continue
			}
			index[filepath.Clean(dir.Path)] = struct{}{}
		}
	}

	return index
}

func (index duplicatedDirectoryIndex) containsParentOf(path string) bool {
	if len(index) == 0 || path == "" {
		return false
	}

	dir := filepath.Clean(filepath.Dir(path))
	for {
		if _, ok := index[dir]; ok {
			return true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func compareDirectoryStructure(dir *Directory, dupMap map[uint64]DuplicatedDirectories) {
	finalizeDirectory(dir)
	addToGroup(dupMap, dir.structureHash, dir)
}

func compareDirectory(dir *Directory, numBlocks int, dupMap map[uint64]DuplicatedDirectories) {
	hash, err := computeDirectoryHash(dir, numBlocks)
	if err != nil {
		return
	}

	addToGroup(dupMap, hash, dir)
}

func computeDirectoryHash(dir *Directory, numBlocks int) (uint64, error) {
	finalizeDirectory(dir)
	if contentHash, ok := cachedDirectoryContentHash(dir, numBlocks); ok {
		return contentHash, nil
	}

	if dir.contentHashes == nil {
		dir.contentHashes = make(map[int]uint64)
	}

	hash := crcPool.Get().(hash.Hash64)
	defer func() {
		hash.Reset()
		crcPool.Put(hash)
	}()

	for _, file := range dir.Files {
		fileHash, err := computeFileHash(file, numBlocks)
		if err != nil {
			return 0, err
		}

		hash.Write(fileEntryMarker)
		writeHashString(hash, file.Info.Name())
		writeHashUint64(hash, uint64(file.Info.Size()))
		writeHashUint64(hash, fileHash)
	}

	for _, child := range dir.Children {
		childHash, err := computeDirectoryHash(child, numBlocks)
		if err != nil {
			return 0, err
		}

		hash.Write(directoryEntryMarker)
		writeHashString(hash, filepath.Base(child.Path))
		writeHashUint64(hash, uint64(child.fileCount))
		writeHashUint64(hash, childHash)
	}

	contentHash := hash.Sum64()
	dir.contentHashes[numBlocks] = contentHash
	storeDirectoryHash(dir, numBlocks, contentHash)
	return contentHash, nil
}

type directoryHashGroup struct {
	dirs []*Directory
}

func directoryHashGroups(dupDirs map[uint64]DuplicatedDirectories) []directoryHashGroup {
	groups := make([]directoryHashGroup, 0, len(dupDirs))
	for _, duplicated := range dupDirs {
		groups = append(groups, directoryHashGroup{
			dirs: duplicated.ListDuplicated,
		})
	}

	return groups
}

func collectFilesForUncachedDirectories(dirs []*Directory, numBlocks int) []File {
	seen := make(map[string]struct{})
	var files []File
	for _, dir := range dirs {
		files = appendUncachedDirectoryFiles(files, dir, numBlocks, seen)
	}
	return files
}

func compareDirectoryGroups(dupDirs map[uint64]DuplicatedDirectories, numBlocks int, dupMap map[uint64]DuplicatedDirectories, verbose bool) {
	dirGroups := directoryHashGroups(dupDirs)
	for groupIndex, group := range dirGroups {
		if compareCachedDirectoryGroup(group.dirs, numBlocks, dupMap) {
			if verbose {
				fmt.Printf("[+] %d / %d done\r", groupIndex+1, len(dirGroups))
			}
			continue
		}

		files := collectFilesForUncachedDirectories(group.dirs, numBlocks)
		hashFiles(files, numBlocks)
		for _, dir := range dirGroups[groupIndex].dirs {
			compareDirectory(dir, numBlocks, dupMap)
		}
		if verbose {
			fmt.Printf("[+] %d / %d done\r", groupIndex+1, len(dirGroups))
		}
	}
}

func compareCachedDirectoryGroup(dirs []*Directory, numBlocks int, dupMap map[uint64]DuplicatedDirectories) bool {
	hashes := make([]uint64, len(dirs))
	for i, dir := range dirs {
		hash, ok := cachedDirectoryContentHash(dir, numBlocks)
		if !ok {
			return false
		}
		hashes[i] = hash
	}

	for i, dir := range dirs {
		addToGroup(dupMap, hashes[i], dir)
	}

	return true
}

func appendUncachedDirectoryFiles(files []File, dir *Directory, numBlocks int, seen map[string]struct{}) []File {
	if dir == nil {
		return files
	}
	finalizeDirectory(dir)
	if _, ok := cachedDirectoryContentHash(dir, numBlocks); ok {
		return files
	}

	for _, file := range dir.Files {
		if _, ok := seen[file.Path]; ok {
			continue
		}
		seen[file.Path] = struct{}{}
		files = append(files, file)
	}

	for _, child := range dir.Children {
		files = appendUncachedDirectoryFiles(files, child, numBlocks, seen)
	}

	return files
}

func writeHashString(hash hash.Hash64, value string) {
	writeHashUint64(hash, uint64(len(value)))
	hash.Write([]byte(value))
}

func writeHashUint64(hash hash.Hash64, value uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], value)
	hash.Write(buf[:])
}

func writeHashInt64(hash hash.Hash64, value int64) {
	writeHashUint64(hash, uint64(value))
}

func doPartialDirectoryCompare(verbose bool) {
	dirsBefore := countItemsInGroups(dupDirFileCount)
	removeSingletonGroups(dupDirFileCount)

	for _, v := range dupDirFileCount {
		for _, dir := range v.ListDuplicated {
			compareDirectoryStructure(dir, structureDuplicatedDirectories)
		}
	}
	removeSingletonGroups(structureDuplicatedDirectories)

	dirsAfter := countItemsInGroups(structureDuplicatedDirectories)

	if verbose {
		fmt.Printf("[+] From %d directories, %d need to be rechecked (%d sets).\n",
			dirsBefore, dirsAfter, len(structureDuplicatedDirectories))
		fmt.Printf("[+] Starting directory stage 2 / 3.\n")
	}

	compareDirectoryGroups(structureDuplicatedDirectories, partialHashBlocks, partialDuplicatedDirectories, verbose)

	reportStageCompletion(verbose, len(structureDuplicatedDirectories) > 0, "[+] Directory stage 2 / 3 completed.")
}

func doFullDirectoryCompare(verbose bool) {
	dirsBefore := countItemsInGroups(partialDuplicatedDirectories)
	removeSingletonGroups(partialDuplicatedDirectories)
	dirsAfter := countItemsInGroups(partialDuplicatedDirectories)

	if verbose {
		fmt.Printf("[+] From %d directories, %d need to be rechecked (%d sets).\n",
			dirsBefore, dirsAfter, len(partialDuplicatedDirectories))
		fmt.Printf("[+] Starting directory stage 3 / 3.\n")
	}

	compareDirectoryGroups(partialDuplicatedDirectories, fullHashBlocks, DuplicatedDirectoriesFound, verbose)
	removeSingletonGroups(DuplicatedDirectoriesFound)
	removeNestedDuplicatedDirectories()

	reportStageCompletion(
		verbose,
		len(partialDuplicatedDirectories) > 0,
		"[+] Directory stage 3 / 3 completed.",
	)
}

func removeNestedDuplicatedDirectories() {
	if len(DuplicatedDirectoriesFound) < 2 {
		return
	}

	pathIndex := newDuplicatedDirectoryPathIndex(DuplicatedDirectoriesFound)
	for candidateHash, candidate := range DuplicatedDirectoriesFound {
		if duplicatedDirectoryGroupIsNested(candidateHash, candidate.ListDuplicated, pathIndex) {
			delete(DuplicatedDirectoriesFound, candidateHash)
		}
	}
}

type duplicatedDirectoryPathIndex map[string]uint64

type nestedDirectoryCoverage struct {
	childCount  int
	parentPaths map[string]struct{}
}

func newDuplicatedDirectoryPathIndex(dupDirs map[uint64]DuplicatedDirectories) duplicatedDirectoryPathIndex {
	index := make(duplicatedDirectoryPathIndex)
	for hash, duplicated := range dupDirs {
		for _, dir := range duplicated.ListDuplicated {
			if dir == nil || dir.Path == "" {
				continue
			}
			index[filepath.Clean(dir.Path)] = hash
		}
	}

	return index
}

func duplicatedDirectoryGroupIsNested(candidateHash uint64, dirs []*Directory, pathIndex duplicatedDirectoryPathIndex) bool {
	if len(dirs) == 0 || len(pathIndex) == 0 {
		return false
	}

	coverage := make(map[uint64]*nestedDirectoryCoverage)
	seenInChild := make(map[uint64]int)
	for childIndex, dir := range dirs {
		if dir == nil || dir.Path == "" {
			return false
		}

		for parentPath := filepath.Clean(filepath.Dir(dir.Path)); ; parentPath = filepath.Dir(parentPath) {
			parentHash, ok := pathIndex[parentPath]
			if ok && parentHash != candidateHash {
				cover := coverage[parentHash]
				if cover == nil {
					cover = &nestedDirectoryCoverage{parentPaths: make(map[string]struct{})}
					coverage[parentHash] = cover
				}
				cover.parentPaths[parentPath] = struct{}{}
				if seenInChild[parentHash] != childIndex+1 {
					cover.childCount++
					seenInChild[parentHash] = childIndex + 1
				}
			}

			nextParent := filepath.Dir(parentPath)
			if nextParent == parentPath {
				break
			}
		}
	}

	for _, cover := range coverage {
		if cover.childCount == len(dirs) && len(cover.parentPaths) >= 2 {
			return true
		}
	}

	return false
}
