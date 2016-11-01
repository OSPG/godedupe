package main

import (
	"crypto/md5"
	"fmt"
	"os"
)

type File struct {
	path string
	f    os.FileInfo
}

type Duplicated struct {
	list_duplicated []File
}

var duplicated_files map[[md5.Size]byte]Duplicated = make(map[[md5.Size]byte]Duplicated)

func CompareFile(f os.FileInfo, path string) {
	file := File{
		path,
		f,
	}

	tmp, err := ComputeMD5(path)
	if err != nil {
		return
	}

	//Convert from slice to array
	var hash [md5.Size]byte
	copy(hash[:], tmp)

	//Check if it exist a duplicated of the current file
	if val, ok := duplicated_files[hash]; ok {
		fmt.Println()
		fmt.Println("Duplicated file: " + path)
		fmt.Println("First duplicated file: " + val.list_duplicated[0].path)
		fmt.Println()
		val.list_duplicated = append(val.list_duplicated, file)
	} else {
		var file_slice []File
		file_slice = append(file_slice, file)
		d := Duplicated{
			file_slice,
		}

		duplicated_files[hash] = d
	}
}
