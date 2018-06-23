package main

import (
	"bufio"
	"fmt"
	"hash/crc64"
	"io"
	"os"
	"sync"
)

// File contains the path to the file and her info
type File struct {
	path string
	info os.FileInfo
}

// Duplicated maintain an array of duplicated files
type Duplicated struct {
	listDuplicated []File
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

	hash := crc64.New(crc64.MakeTable(crc64.ECMA))
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

	return hash.Sum64(), nil
}

// compareFile checks if the hash of the "path" file are in the map, in that
// case, append it to the listDuplicated otherwise creates a new Duplicated
// for storing future duplicates of the current file
func compareFile(file File, numBlocks int, dupMap map[uint64]Duplicated) {
	hash, err := computeHash(file.path, numBlocks)
	if err != nil {
		return
	}

	//Check if exist a duplicated of the current file
	if val, ok := dupMap[hash]; ok {
		val.listDuplicated = append(val.listDuplicated, file)
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
		dups := len(v.listDuplicated) - 1
		if dups == 0 {
			delete(dupMap, k)
		}
	}
}

// AddFile append files to the dupFileSize map to be compared later
func AddFile(file File) {
	size := file.info.Size()
	if val, ok := dupFileSize[size]; ok {
		val.listDuplicated = append(val.listDuplicated, file)
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
func ValidateDuplicatedFiles() {
	doCompare()
	obtainDuplicates()
}

// make the full file comparison
func obtainDuplicates() {
	filesBefore := 0
	for _, v := range partialDuplicatedFiles {
		filesBefore += len(v.listDuplicated)
	}

	cleanUnmarried(partialDuplicatedFiles)

	filesAfter := 0
	for _, v := range partialDuplicatedFiles {
		filesAfter += len(v.listDuplicated)
	}

	if !opt.quiet {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(partialDuplicatedFiles))
		fmt.Printf("[+] Starting stage 3 / 3.\n")
	}

	i := 0
	for _, v := range partialDuplicatedFiles {
		for _, f := range v.listDuplicated {
			compareFile(f, 0, DuplicatedFiles)
		}
		if !opt.quiet {
			i++
			fmt.Printf("[+] %d / %d done\r",
				i, len(partialDuplicatedFiles))
		}
	}
	cleanUnmarried(DuplicatedFiles)

	if !opt.quiet {
		fmt.Printf("[+] Stage 3 / 3 completed.\n\n")
	}
}

// make a partial file comparison
func doCompare() {
	filesBefore := 0
	for _, v := range dupFileSize {
		filesBefore += len(v.listDuplicated)
	}

	for k, v := range dupFileSize {
		dups := len(v.listDuplicated) - 1
		if dups == 0 {
			delete(dupFileSize, k)
		}
	}

	filesAfter := 0
	for _, v := range dupFileSize {
		filesAfter += len(v.listDuplicated)
	}
	if !opt.quiet {
		fmt.Printf("[+] From %d files, %d need to be rechecked (%d sets).\n",
			filesBefore, filesAfter, len(dupFileSize))
		fmt.Printf("[+] Starting stage 2 / 3.\n")
	}

	i := 0
	for _, v := range dupFileSize {
		for _, f := range v.listDuplicated {
			compareFile(f, 1, partialDuplicatedFiles)
		}
		if !opt.quiet {
			i++
			fmt.Printf("[+] %d / %d done\r",
				i, len(dupFileSize))
		}
	}
	if !opt.quiet {
		fmt.Printf("\n[+] Stage 2 / 3 completed.\n")
	}
}
