package main

import (
	"crypto/md5"
	"fmt"
	"os"
)

type File struct {
	path string
	info os.FileInfo
}

type Duplicated struct {
	list_duplicated []File
}

var Duplicated_files map[[md5.Size]byte]Duplicated = make(map[[md5.Size]byte]Duplicated)

// CompareFile checks if the hash of the "path" file are in the map, in that case, append it to the list_duplicated
// otherwise creates a new Duplicated for storing future duplicates of the current file
func CompareFile(path string, f os.FileInfo) {
	tmp, err := ComputeMD5(path)
	if err != nil {
		return
	}

	//Convert from slice to array
	var hash [md5.Size]byte
	copy(hash[:], tmp)

	file := File{
		path,
		f,
	}

	//Check if it exist a duplicated of the current file
	if val, ok := Duplicated_files[hash]; ok {
		fmt.Println()
		fmt.Println("Duplicated file: " + path)
		fmt.Println("First duplicated file: " + val.list_duplicated[0].path)
		fmt.Println()
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
