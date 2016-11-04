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

var Duplicated_files map[[blakeSize]byte]Duplicated = make(map[[blakeSize]byte]Duplicated)

// CompareFile checks if the hash of the "path" file are in the map, in that case, append it to the list_duplicated
// otherwise creates a new Duplicated for storing future duplicates of the current file
func CompareFile(file File) {
	tmp, err := ComputeHash(file.path)
	if err != nil {
		return
	}

	//Convert from slice to array
	var hash [blakeSize]byte
	copy(hash[:], tmp)

	//Check if it exist a duplicated of the current file
	if val, ok := Duplicated_files[hash]; ok {
		val.list_duplicated = append(val.list_duplicated, file)
		Duplicated_files[hash] = val
	} else {
		var file_slice []File
		file_slice = append(file_slice, file)
		d := Duplicated{
			file_slice,
		}

		Duplicated_files[hash] = d
	}
}
