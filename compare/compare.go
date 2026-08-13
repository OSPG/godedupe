package compare

import (
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"os"
	"runtime"
	"sync"
)

// File contains the path to the file and its info.
type File struct {
	Path string
	Info os.FileInfo
}

// Duplicated stores files with matching comparison keys.
type Duplicated = duplicateGroup[File]

var (
	dupFileSize            = make(map[int64]Duplicated)
	partialDuplicatedFiles = make(map[uint64]Duplicated)
	// DuplicatedFiles stores all known duplicated files.
	DuplicatedFiles = make(map[uint64]Duplicated)
	fileHashes      = make(map[fileHashKey]uint64)
	fileHashesMu    sync.RWMutex
)

const (
	fullHashBlocks                   = 0
	bufferSize                       = 1024
	fullHashBufferSize               = 32 * 1024
	partialHashBlocks                = 1
	defaultLargeFileHashCacheMinSize = 500 * 1024 * 1024
)

var crcTable = crc64.MakeTable(crc64.ECMA)

var largeFileHashCacheMinSize int64 = defaultLargeFileHashCacheMinSize

type fileHashKey struct {
	path      string
	numBlocks int
}

var (
	partialHashBytePool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, bufferSize)
		},
	}
	fullHashBytePool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, fullHashBufferSize)
		},
	}
	crcPool = &sync.Pool{
		New: func() interface{} {
			return crc64.New(crcTable)
		},
	}
)

func cachedFileHash(key fileHashKey) (uint64, bool) {
	fileHashesMu.RLock()
	hash, ok := fileHashes[key]
	fileHashesMu.RUnlock()
	return hash, ok
}

func storeFileHash(key fileHashKey, hash uint64) {
	fileHashesMu.Lock()
	fileHashes[key] = hash
	fileHashesMu.Unlock()
}

func storeSmallFileHashes(path string, hash uint64) {
	fileHashesMu.Lock()
	fileHashes[fileHashKey{path: path, numBlocks: partialHashBlocks}] = hash
	fileHashes[fileHashKey{path: path, numBlocks: fullHashBlocks}] = hash
	fileHashesMu.Unlock()
}

func cachedSmallFileHash(file File) (uint64, bool) {
	partialKey := fileHashKey{path: file.Path, numBlocks: partialHashBlocks}
	fullKey := fileHashKey{path: file.Path, numBlocks: fullHashBlocks}

	fileHashesMu.RLock()
	partialHash, hasPartialHash := fileHashes[partialKey]
	fullHash, hasFullHash := fileHashes[fullKey]
	fileHashesMu.RUnlock()

	switch {
	case hasPartialHash:
		storeFileHash(fullKey, partialHash)
		return partialHash, true
	case hasFullHash:
		storeFileHash(partialKey, fullHash)
		return fullHash, true
	default:
		return 0, false
	}
}

// computeHash calculates either a full file hash or the first numBlocks blocks.
func computeHash(filename string, numBlocks int) (uint64, error) {
	key := fileHashKey{path: filename, numBlocks: numBlocks}
	if hash, ok := cachedFileHash(key); ok {
		return hash, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	hash := crcPool.Get().(hash.Hash64)
	var buf []byte
	fullHash := numBlocks == fullHashBlocks
	if fullHash {
		buf = fullHashBytePool.Get().([]byte)
		defer fullHashBytePool.Put(buf)
	} else {
		buf = partialHashBytePool.Get().([]byte)
		defer partialHashBytePool.Put(buf)
	}
	defer func() {
		hash.Reset()
		crcPool.Put(hash)
	}()

	for blocksRead := 0; fullHash || blocksRead < numBlocks; blocksRead++ {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	s := hash.Sum64()
	storeFileHash(key, s)

	return s, nil
}

func computeFileHash(file File, numBlocks int) (uint64, error) {
	if file.Info.Size() <= bufferSize {
		if hash, ok := cachedSmallFileHash(file); ok {
			return hash, nil
		}
	}
	if hash, ok := cachedLargeFileHash(file, numBlocks); ok {
		storeFileHash(fileHashKey{path: file.Path, numBlocks: numBlocks}, hash)
		return hash, nil
	}

	hash, err := computeHash(file.Path, numBlocks)
	if err != nil {
		return 0, err
	}

	if file.Info.Size() <= bufferSize {
		storeSmallFileHashes(file.Path, hash)
	}
	storeLargeFileHash(file, numBlocks, hash)

	return hash, nil
}

type fileHashResult struct {
	file File
	hash uint64
	ok   bool
}

type fileHashGroup struct {
	files     []File
	numBlocks int
}

type indexedFileHashGroupResult struct {
	groupIndex int
	results    []fileHashResult
}

func hashWorkerCount(jobCount int) int {
	workers := runtime.GOMAXPROCS(0)
	if workers > jobCount {
		workers = jobCount
	}
	if workers < 1 {
		workers = 1
	}

	return workers
}

func hashFileGroups(groups []fileHashGroup, verbose bool, onGroupComplete func(int, []fileHashResult)) {
	if len(groups) == 0 {
		return
	}

	workers := hashWorkerCount(len(groups))
	jobs := make(chan int)
	groupResults := make(chan indexedFileHashGroupResult, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for groupIndex := range jobs {
				group := groups[groupIndex]
				results := make([]fileHashResult, len(group.files))
				for fileIndex, file := range group.files {
					results[fileIndex] = hashFile(file, group.numBlocks)
				}
				groupResults <- indexedFileHashGroupResult{
					groupIndex: groupIndex,
					results:    results,
				}
			}
		}()
	}

	go func() {
		for groupIndex := range groups {
			jobs <- groupIndex
		}
		close(jobs)
		wg.Wait()
		close(groupResults)
	}()

	completed := 0
	for groupResult := range groupResults {
		if onGroupComplete != nil {
			onGroupComplete(groupResult.groupIndex, groupResult.results)
		}
		completed++
		if verbose {
			fmt.Printf("[+] %d / %d done\r", completed, len(groups))
		}
	}
}

func compareSizeFileGroups(groups []fileHashGroup, verbose bool) {
	hashFileGroups(groups, verbose, func(groupIndex int, results []fileHashResult) {
		dupMap := partialDuplicatedFiles
		if groups[groupIndex].numBlocks == fullHashBlocks {
			dupMap = DuplicatedFiles
		}
		for _, result := range results {
			if result.ok {
				addToGroup(dupMap, result.hash, result.file)
			}
		}
	})
}

func sizeFileHashGroups(groups map[int64]Duplicated) []fileHashGroup {
	hashGroups := make([]fileHashGroup, 0, len(groups))
	for size, group := range groups {
		numBlocks := partialHashBlocks
		if size <= bufferSize {
			numBlocks = fullHashBlocks
		}

		hashGroups = append(hashGroups, fileHashGroup{
			files:     group.ListDuplicated,
			numBlocks: numBlocks,
		})
	}
	return hashGroups
}

func duplicatedFileHashGroups(groups map[uint64]Duplicated, numBlocks int) []fileHashGroup {
	hashGroups := make([]fileHashGroup, 0, len(groups))
	for _, group := range groups {
		hashGroups = append(hashGroups, fileHashGroup{
			files:     group.ListDuplicated,
			numBlocks: numBlocks,
		})
	}
	return hashGroups
}

func hashFile(file File, numBlocks int) fileHashResult {
	hash, err := computeFileHash(file, numBlocks)
	if err != nil {
		return fileHashResult{}
	}

	return fileHashResult{file: file, hash: hash, ok: true}
}

func reportStageCompletion(verbose bool, progressPrinted bool, message string) {
	if !verbose {
		return
	}

	if progressPrinted {
		fmt.Println()
	}
	fmt.Println(message)
}

// AddFile appends files to the size groups to be compared later.
func AddFile(file File) {
	addToGroup(dupFileSize, file.Info.Size(), file)
}

// ValidateDuplicatedFiles validates duplicate candidates with partial and full file hashes.
func ValidateDuplicatedFiles(verbose bool) {
	doPartialFileCompare(verbose)
	doFullFileCompare(verbose)
}

func doFullFileCompare(verbose bool) {
	filesBefore := countItemsInGroups(partialDuplicatedFiles)
	removeSingletonGroups(partialDuplicatedFiles)
	filesAfter := countItemsInGroups(partialDuplicatedFiles)

	if verbose {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(partialDuplicatedFiles))
		fmt.Printf("[+] Starting stage 3 / 3.\n")
	}

	fileGroups := duplicatedFileHashGroups(partialDuplicatedFiles, fullHashBlocks)
	hashFileGroups(fileGroups, verbose, func(_ int, results []fileHashResult) {
		for _, result := range results {
			if result.ok {
				addToGroup(DuplicatedFiles, result.hash, result.file)
			}
		}
	})
	removeSingletonGroups(DuplicatedFiles)

	reportStageCompletion(verbose, len(fileGroups) > 0, "[+] Stage 3 / 3 completed.")
}

func doPartialFileCompare(verbose bool) {
	filesBefore := countItemsInGroups(dupFileSize)
	removeSingletonGroups(dupFileSize)
	filesAfter := countItemsInGroups(dupFileSize)
	if verbose {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(dupFileSize))
		fmt.Printf("[+] Starting stage 2 / 3.\n")
	}

	fileGroups := sizeFileHashGroups(dupFileSize)
	compareSizeFileGroups(fileGroups, verbose)
	reportStageCompletion(verbose, len(fileGroups) > 0, "[+] Stage 2 / 3 completed.")
}
