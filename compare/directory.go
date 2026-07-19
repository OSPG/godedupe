package compare

import (
	"encoding/binary"
	"fmt"
	"hash"
	"path/filepath"
	"sort"
	"strings"
)

// Directory contains a comparable directory tree.
type Directory struct {
	Path     string
	Files    []File
	Children []*Directory

	fileCount     int64
	size          int64
	structureHash uint64
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

	hash := crcPool.Get().(hash.Hash64)
	defer func() {
		hash.Reset()
		crcPool.Put(hash)
	}()

	for _, file := range dir.Files {
		dir.fileCount++
		dir.size += file.Info.Size()
		hash.Write(fileEntryMarker)
		writeHashString(hash, file.Info.Name())
		writeHashUint64(hash, uint64(file.Info.Size()))
	}

	for _, child := range dir.Children {
		finalizeDirectory(child)
		dir.fileCount += child.fileCount
		dir.size += child.size
		if child.fileCount == 0 {
			continue
		}
		hash.Write(directoryEntryMarker)
		writeHashString(hash, filepath.Base(child.Path))
		writeHashUint64(hash, uint64(child.fileCount))
		writeHashUint64(hash, child.structureHash)
	}

	dir.structureHash = hash.Sum64()
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
func FilterFilesCoveredByDirectories(dupFiles map[uint64]Duplicated, dupDirs map[uint64]DuplicatedDirectories) map[uint64]Duplicated {
	filtered := make(map[uint64]Duplicated)

	for fileHash, duplicated := range dupFiles {
		var files []File
		for _, file := range duplicated.ListDuplicated {
			if !pathInDuplicatedDirectories(file.Path, dupDirs) {
				files = append(files, file)
			}
		}

		if len(files) > 1 {
			filtered[fileHash] = Duplicated{ListDuplicated: files}
		}
	}

	return filtered
}

func pathInDuplicatedDirectories(path string, dupDirs map[uint64]DuplicatedDirectories) bool {
	for _, duplicated := range dupDirs {
		for _, dir := range duplicated.ListDuplicated {
			if isChildPath(dir.Path, path) {
				return true
			}
		}
	}

	return false
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

	if dir.contentHashes == nil {
		dir.contentHashes = make(map[int]uint64)
	}
	if contentHash, ok := dir.contentHashes[numBlocks]; ok {
		return contentHash, nil
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
		if child.fileCount == 0 {
			continue
		}

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
	return contentHash, nil
}

func precomputeDirectoryFileHashes(dupDirs map[uint64]DuplicatedDirectories, numBlocks int) {
	files := collectDirectoryFiles(dupDirs)
	hashFilesIntoCache(files, numBlocks)
}

func collectDirectoryFiles(dupDirs map[uint64]DuplicatedDirectories) []File {
	seen := make(map[string]struct{})
	var files []File

	for _, duplicated := range dupDirs {
		for _, dir := range duplicated.ListDuplicated {
			files = appendDirectoryFiles(files, dir, seen)
		}
	}

	return files
}

func appendDirectoryFiles(files []File, dir *Directory, seen map[string]struct{}) []File {
	if dir == nil {
		return files
	}
	finalizeDirectory(dir)

	for _, file := range dir.Files {
		if _, ok := seen[file.Path]; ok {
			continue
		}
		seen[file.Path] = struct{}{}
		files = append(files, file)
	}

	for _, child := range dir.Children {
		files = appendDirectoryFiles(files, child, seen)
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

	i := 0
	precomputeDirectoryFileHashes(structureDuplicatedDirectories, partialHashBlocks)
	for _, v := range structureDuplicatedDirectories {
		for _, dir := range v.ListDuplicated {
			compareDirectory(dir, partialHashBlocks, partialDuplicatedDirectories)
		}
		i++
		reportStageProgress(verbose, i, len(structureDuplicatedDirectories))
	}

	if verbose {
		fmt.Printf("\n[+] Directory stage 2 / 3 completed.\n")
	}
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

	i := 0
	precomputeDirectoryFileHashes(partialDuplicatedDirectories, fullHashBlocks)
	for _, v := range partialDuplicatedDirectories {
		for _, dir := range v.ListDuplicated {
			compareDirectory(dir, fullHashBlocks, DuplicatedDirectoriesFound)
		}
		i++
		reportStageProgress(verbose, i, len(partialDuplicatedDirectories))
	}
	removeSingletonGroups(DuplicatedDirectoriesFound)
	removeNestedDuplicatedDirectories()

	if verbose {
		fmt.Printf("[+] Directory stage 3 / 3 completed.\n\n")
	}
}

func removeNestedDuplicatedDirectories() {
	for candidateHash, candidate := range DuplicatedDirectoriesFound {
		for parentHash, parent := range DuplicatedDirectoriesFound {
			if parentHash == candidateHash {
				continue
			}

			if duplicatedDirectoriesAreNested(candidate.ListDuplicated, parent.ListDuplicated) {
				delete(DuplicatedDirectoriesFound, candidateHash)
				break
			}
		}
	}
}

func duplicatedDirectoriesAreNested(children, parents []*Directory) bool {
	coveredParents := make(map[string]bool)

	for _, child := range children {
		matchedParent := false
		for _, parent := range parents {
			if isChildPath(parent.Path, child.Path) {
				matchedParent = true
				coveredParents[parent.Path] = true
				break
			}
		}

		if !matchedParent {
			return false
		}
	}

	return len(coveredParents) >= 2
}

func isChildPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}

	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
