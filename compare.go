package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
)

type File struct {
	path string
	f    os.FileInfo
}

type Duplicated struct {
	list_duplicated []File
	hash            [md5.Size]byte
	size            int64
}

//TODO: That should be a map
var duplicated_files []Duplicated

func CompareFile(f os.FileInfo, path string) {
	file := File{
		path,
		f,
	}

	is_found := false

	//Check if it exist a duplicated of the current file
	for _, dup := range duplicated_files {

		if dup.size == f.Size() {
			//TODO: Puting all the file in memory is not cool, it's only a POC
			file_content, err := ioutil.ReadFile(path)
			if err != nil {
				return
			}

			hash := md5.Sum(file_content)

			if dup.hash == hash {
				fmt.Println()
				fmt.Println("Duplicated file: " + path)
				fmt.Println("First duplicated file: " + dup.list_duplicated[0].path)
				fmt.Println()
				dup.list_duplicated = append(dup.list_duplicated, file)
				is_found = true
			}

		}

	}

	//If a duplicated is found we don not need to do anything
	//If it's not found we add it to the list
	if is_found {
		return
	}

	//TODO: Puting all the file in memory is not cool, it's only a POC
	//TODO: Duplicated code is not col either, it should be a different function
	file_content, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	hash := md5.Sum(file_content)

	var file_slice []File
	file_slice = append(file_slice, file)
	d := Duplicated{
		file_slice,
		hash,
		f.Size(),
	}

	duplicated_files = append(duplicated_files, d)
}
