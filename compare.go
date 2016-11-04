package main

import (
	"os"
)

type File struct {
	path string
	info os.FileInfo
}

type Duplicated struct {
	list_duplicated []File
}

// By default blake2b uses a size of 64, but we will use New256 not New512 so size should be 32
const blakeSize int = 32

var partialDuplicatedFiles map[[blakeSize]byte]Duplicated = make(map[[blakeSize]byte]Duplicated)
var DuplicatedFiles map[[blakeSize]byte]Duplicated = make(map[[blakeSize]byte]Duplicated)

// compareFile checks if the hash of the "path" file are in the map, in that case, append it to the list_duplicated
// otherwise creates a new Duplicated for storing future duplicates of the current file
func compareFile(file File, numBlocks int, dupMap map[[blakeSize]byte]Duplicated) {
	//fmt.Println(len(dupMap))
	tmp, err := ComputeHash(file.path, numBlocks)
	if err != nil {
		return
	}

	//Convert from slice to array
	var hash [blakeSize]byte
	copy(hash[:], tmp)

	//Check if exist a duplicated of the current file
	if val, ok := dupMap[hash]; ok {
		val.list_duplicated = append(val.list_duplicated, file)
		dupMap[hash] = val
	} else {
		var file_slice []File
		file_slice = append(file_slice, file)
		d := Duplicated{
			file_slice,
		}

		dupMap[hash] = d
	}
}

func cleanUnmarried(dupMap map[[blakeSize]byte]Duplicated) {
	for k, v := range dupMap {
		dups := len(v.list_duplicated) - 1
		if dups == 0 {
			delete(dupMap, k)
		}
	}
}

func ComparePartialFile(file File) {
	//XXX: In theory in Go there are not pass by reference, then why is duplicated_files modified?
	compareFile(file, 1, partialDuplicatedFiles)
}

// Do the full hash of the duplicated_files to avoid false positives
func ValidateDuplicatedFiles() {
	cleanUnmarried(partialDuplicatedFiles)

	for _, v := range partialDuplicatedFiles {
		for _, f := range v.list_duplicated {
			compareFile(f, 0, DuplicatedFiles)
		}
	}

	cleanUnmarried(DuplicatedFiles)

}
