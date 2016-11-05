package main

import (
	"fmt"
	"os"
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

// By default blake2b uses a size of 64, but we will use New256 not New512
// so size should be 32
const blakeSize int = 32

var (
	dupFileSize            = make(map[int64]Duplicated)
	partialDuplicatedFiles = make(map[[blakeSize]byte]Duplicated)
)

// DuplicatedFiles store all know duplicated files
var DuplicatedFiles = make(map[[blakeSize]byte]Duplicated)

// compareFile checks if the hash of the "path" file are in the map, in that
// case, append it to the listDuplicated otherwise creates a new Duplicated
// for storing future duplicates of the current file
func compareFile(file File, numBlocks int, dupMap map[[blakeSize]byte]Duplicated) {
	tmp, err := ComputeHash(file.path, numBlocks)
	if err != nil {
		return
	}

	//Convert from slice to array
	var hash [blakeSize]byte
	copy(hash[:], tmp)

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

func cleanUnmarried(dupMap map[[blakeSize]byte]Duplicated) {
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
	cleanUnmarried(partialDuplicatedFiles)

	var setsDuplicated int
	for _, v := range partialDuplicatedFiles {
		setsDuplicated += len(v.listDuplicated)
	}
	fmt.Printf("[+] From %d sets, %d need to be rechecked\n",
		setsDuplicated, len(partialDuplicatedFiles))
	fmt.Printf("[+] Starting stage 3 / 3.\n")

	i := 0
	for _, v := range partialDuplicatedFiles {
		for _, f := range v.listDuplicated {
			compareFile(f, 0, DuplicatedFiles)
		}
		i++
		fmt.Printf("[+] %d / %d done\r",
			i, len(partialDuplicatedFiles))
	}
	cleanUnmarried(DuplicatedFiles)

	fmt.Printf("\n[+] Stage 3 / 3 completed.\n")
}

// DoCompare make a partial file comparison
func DoCompare() {
	//originalSize := len(dupFileSize)
	for k, v := range dupFileSize {
		dups := len(v.listDuplicated) - 1
		if dups == 0 {
			delete(dupFileSize, k)
		}
	}
	var setsDuplicated int
	for _, v := range dupFileSize {
		setsDuplicated += len(v.listDuplicated)
	}
	fmt.Printf("[+] From %d sets, %d need to be rechecked.\n",
		setsDuplicated, len(dupFileSize))
	fmt.Printf("[+] Starting stage 2 / 3.\n")

	i := 0
	for _, v := range dupFileSize {
		for _, f := range v.listDuplicated {
			compareFile(f, 1, partialDuplicatedFiles)
		}
		i++
		fmt.Printf("[+] %d / %d done\r",
			i, len(dupFileSize))
	}

	fmt.Printf("\n[+] Stage 2 done.\n")
}
