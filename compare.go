package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
)

type File struct {
	path string
	f    os.FileInfo
}

type Duplicated struct {
	list_duplicated []File
}

var duplicated_files map[[md5.Size]byte]Duplicated = make(map[[md5.Size]byte]Duplicated)

var mutex sync.Mutex

func CompareFile(f os.FileInfo, path string) {
	file := File{
		path,
		f,
	}

	//TODO: Puting all the file in memory is not cool, it's only a POC
	file_content, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}

	hash := md5.Sum(file_content)

	//Check if it exist a duplicated of the current file
	mutex.Lock()
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
	mutex.Unlock()

}
