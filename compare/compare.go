package compare

import (
	"bufio"
	"fmt"
	"hash"
	"hash/crc64"
	"io"
	"os"
	"sync"
)

// File contains the path to the file and her info
type File struct {
	Path string
	Info os.FileInfo
}

// Duplicated maintain an array of duplicated files
type Duplicated struct {
	ListDuplicated []File
}

var (
	dupFileSize            = make(map[int64]Duplicated)
	partialDuplicatedFiles = make(map[uint64]Duplicated)
	// DuplicatedFiles store all know duplicated files
	DuplicatedFiles = make(map[uint64]Duplicated)
)

const bufferSize = 1024

var (
	bytePool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, bufferSize)
		},
	}
	crcPool = &sync.Pool{
		New: func() interface{} {
			return crc64.New(crc64.MakeTable(crc64.ECMA))
		},
	}
)

// computeHash calculates the hash for the current file
// if bufferNumber is not zero then we will only hash the first bufferNumber
// blocks (bufferSize)
func computeHash(filename string, bufNumber int) (uint64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	hash := crcPool.Get().(hash.Hash64)
	buf, reader := bytePool.Get().([]byte), bufio.NewReader(file)
	if bufNumber == 0 {
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, err
			}

			hash.Write(buf[:n])
		}
	} else {
		for ; bufNumber > 0; bufNumber-- {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return 0, err
			}

			hash.Write(buf[:n])
		}
	}
	bytePool.Put(buf)
	s := hash.Sum64()
	hash.Reset()
	crcPool.Put(hash)

	return s, nil
}

// compareFile checks if the hash of the "path" file are in the map, in that
// case, append it to the ListDuplicated otherwise creates a new Duplicated
// for storing future duplicates of the current file
func compareFile(file File, numBlocks int, dupMap map[uint64]Duplicated) {
	hash, err := computeHash(file.Path, numBlocks)
	if err != nil {
		return
	}

	//Check if exist a duplicated of the current file
	if val, ok := dupMap[hash]; ok {
		val.ListDuplicated = append(val.ListDuplicated, file)
		dupMap[hash] = val
	} else {
		var fileSlice []File
		fileSlice = append(fileSlice, file)
		d := Duplicated{
			fileSlice,
		}

		dupMap[hash] = d
	}
}

func cleanUnmarried(dupMap map[uint64]Duplicated) {
	for k, v := range dupMap {
		dups := len(v.ListDuplicated) - 1
		if dups == 0 {
			delete(dupMap, k)
		}
	}
}

// AddFile append files to the dupFileSize map to be compared later
func AddFile(file File) {
	size := file.Info.Size()
	if val, ok := dupFileSize[size]; ok {
		val.ListDuplicated = append(val.ListDuplicated, file)
		dupFileSize[size] = val
	} else {
		var fileSlice []File
		fileSlice = append(fileSlice, file)
		d := Duplicated{
			fileSlice,
		}
		dupFileSize[size] = d
	}
}

// ValidateDuplicatedFiles do the full hash of the duplicatedFiles to
// avoid false positives
func ValidateDuplicatedFiles(verbose bool) {
	doPartialCompare(verbose)
	doFullCompare(verbose)
}

// make the full file comparison
func doFullCompare(verbose bool) {
	filesBefore := 0
	for _, v := range partialDuplicatedFiles {
		filesBefore += len(v.ListDuplicated)
	}

	cleanUnmarried(partialDuplicatedFiles)

	filesAfter := 0
	for _, v := range partialDuplicatedFiles {
		filesAfter += len(v.ListDuplicated)
	}

	if verbose {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(partialDuplicatedFiles))
		fmt.Printf("[+] Starting stage 3 / 3.\n")
	}

	i := 0
	for _, v := range partialDuplicatedFiles {
		for _, f := range v.ListDuplicated {
			compareFile(f, 0, DuplicatedFiles)
		}
		if verbose {
			i++
			fmt.Printf("[+] %d / %d done\r",
				i, len(partialDuplicatedFiles))
		}
	}
	cleanUnmarried(DuplicatedFiles)

	if verbose {
		fmt.Printf("[+] Stage 3 / 3 completed.\n\n")
	}
}

// make a partial file comparison
func doPartialCompare(verbose bool) {
	filesBefore := 0
	for _, v := range dupFileSize {
		filesBefore += len(v.ListDuplicated)
	}

	for k, v := range dupFileSize {
		dups := len(v.ListDuplicated) - 1
		if dups == 0 {
			delete(dupFileSize, k)
		}
	}

	filesAfter := 0
	for _, v := range dupFileSize {
		filesAfter += len(v.ListDuplicated)
	}
	if verbose {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(dupFileSize))
		fmt.Printf("[+] Starting stage 2 / 3.\n")
	}

	i := 0
	for _, v := range dupFileSize {
		for _, f := range v.ListDuplicated {
			// If file size is less than bufferSize the partial
			// comparison will hash the entire file so we can skip this step
			// and put the result directly on the final map
			if f.Info.Size() <= bufferSize {
				compareFile(f, 0, DuplicatedFiles)
			} else {
				compareFile(f, 1, partialDuplicatedFiles)
			}
		}
		if verbose {
			i++
			fmt.Printf("[+] %d / %d done\r",
				i, len(dupFileSize))
		}
	}
	if verbose {
		fmt.Printf("\n[+] Stage 2 / 3 completed.\n")
	}
}
