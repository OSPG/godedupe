package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var opt Options

var countDirs int
var countFiles int

func update(f os.FileInfo) {
	if f.IsDir() {
		countDirs++
	} else {
		countFiles++
	}
}

// checkFile checks if we have to add this file.
// Returns true if we have to recurse or false if we don't
func checkFile(path string, f os.FileInfo) bool {
	if opt.excludeEmptyFiles && !f.IsDir() && f.Size() == 0 {
		return false
	}
	if opt.excludeHiddenFiles && strings.HasPrefix(f.Name(), ".") {
		// hidden file or directory
		return false
	}
	if opt.ignoreSymLinks && f.Mode()&os.ModeSymlink != 0 {
		return false
	}

	update(f)

	// only make hash for files, skip dirs
	if !f.IsDir() {
		go CompareFile(f, path)
	}

	fmt.Printf("[+] Analyzed: %v directories and %v files\r",
		countDirs, countFiles)

	//fmt.Println(path)

	return true
}

func readDir(s string) error {
	files, err := ioutil.ReadDir(s)
	if err != nil {
		return err
	}

	if opt.excludeEmptyDir && len(files) == 0 {
		return nil
	}

	for _, file := range files {
		path := s + "/" + file.Name()
		recurse := checkFile(path, file)

		if recurse && file.IsDir() {
			readDir(path)
		}
	}
	return nil
}

// Start the program with the current options. Options param is read only
func Start(options Options) {
	opt = options

	if info, err := os.Stat(opt.currentDir); err == nil && !info.IsDir() {
		fmt.Printf("[-] %s is not a valid directory", info.Name())
		return
	}
	fmt.Println("[+] Starting in directory:", opt.currentDir)

	err := readDir(opt.currentDir)
	if err != nil {
		fmt.Println("[-]", err)
	}

	fmt.Println()
}
