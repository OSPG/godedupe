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

type fileHashGroupState struct {
	results   []fileHashResult
	remaining int
}

type fileHashJob struct {
	groupIndex int
	fileIndex  int
	file       File
	numBlocks  int
}

type indexedFileHashResult struct {
	groupIndex int
	fileIndex  int
	result     fileHashResult
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
	progress := newSetProgress(verbose, len(groups))
	hashFileGroupsWithProgress(groups, onGroupComplete, progress.doneGroup)
}

type setProgress struct {
	verbose bool
	done    int
	total   int
}

func newSetProgress(verbose bool, total int) *setProgress {
	if total == 0 {
		return nil
	}

	return &setProgress{verbose: verbose, total: total}
}

func (progress *setProgress) doneGroup() {
	if progress == nil {
		return
	}

	progress.done++
	reportStageProgress(progress.verbose, progress.done, progress.total)
}

func hashFileGroupsWithProgress(groups []fileHashGroup, onGroupComplete func(int, []fileHashResult), onProgress func()) {
	if len(groups) == 0 {
		return
	}

	states, totalFiles := newFileHashGroupStates(groups)
	completeGroup := func(groupIndex int) {
		if onGroupComplete != nil {
			onGroupComplete(groupIndex, states[groupIndex].results)
		}
		if onProgress != nil {
			onProgress()
		}
	}

	for groupIndex, state := range states {
		if state.remaining == 0 {
			completeGroup(groupIndex)
		}
	}
	if totalFiles == 0 {
		return
	}

	workers := hashWorkerCount(totalFiles)
	jobs := make(chan fileHashJob)
	resultCh := make(chan indexedFileHashResult, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				resultCh <- indexedFileHashResult{
					groupIndex: job.groupIndex,
					fileIndex:  job.fileIndex,
					result:     hashFile(job.file, job.numBlocks),
				}
			}
		}()
	}

	go func() {
		for groupIndex, group := range groups {
			for fileIndex, file := range group.files {
				jobs <- fileHashJob{
					groupIndex: groupIndex,
					fileIndex:  fileIndex,
					file:       file,
					numBlocks:  group.numBlocks,
				}
			}
		}
		close(jobs)
		wg.Wait()
		close(resultCh)
	}()

	for result := range resultCh {
		state := &states[result.groupIndex]
		state.results[result.fileIndex] = result.result
		state.remaining--
		if state.remaining == 0 {
			completeGroup(result.groupIndex)
		}
	}
}

func newFileHashGroupStates(groups []fileHashGroup) ([]fileHashGroupState, int) {
	states := make([]fileHashGroupState, len(groups))
	totalFiles := 0

	for i, group := range groups {
		states[i] = fileHashGroupState{
			results:   make([]fileHashResult, len(group.files)),
			remaining: len(group.files),
		}
		totalFiles += len(group.files)
	}

	return states, totalFiles
}

func hashFileGroupResults(results []fileHashResult, dupMap map[uint64]Duplicated) {
	for _, result := range results {
		if result.ok {
			addToGroup(dupMap, result.hash, result.file)
		}
	}
}

func compareFileGroups(groups []fileHashGroup, dupMap map[uint64]Duplicated, verbose bool) {
	hashFileGroups(groups, verbose, func(_ int, results []fileHashResult) {
		hashFileGroupResults(results, dupMap)
	})
}

func compareSizeFileGroups(groups []fileHashGroup, verbose bool) {
	hashFileGroups(groups, verbose, func(groupIndex int, results []fileHashResult) {
		dupMap := partialDuplicatedFiles
		if groups[groupIndex].numBlocks == fullHashBlocks {
			dupMap = DuplicatedFiles
		}
		hashFileGroupResults(results, dupMap)
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

func reportStageProgress(verbose bool, done int, total int) {
	if verbose {
		fmt.Printf("[+] %d / %d done\r", done, total)
	}
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
	compareFileGroups(fileGroups, DuplicatedFiles, verbose)
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
