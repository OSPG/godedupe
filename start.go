package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

func visit(path string, f os.FileInfo, err error) error {
	if err != nil {
		fmt.Println("[-]", err)
	}
	if opt.excludeEmptyFiles && !f.IsDir() && f.Size() == 0 {
		return nil
	}
	if opt.excludeEmptyDir && f.IsDir() {
		files, err := ioutil.ReadDir(path)
		if err != nil {
			fmt.Println("[-]", err)
		}
		if len(files) == 0 {
			return nil
		}
	}
	if opt.excludeHiddenFiles && strings.HasPrefix(f.Name(), ".") {
		// hidden file or directory
		return nil
	}
	if opt.ignoreSymLinks && f.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	update(f)

	// only make hash for files, skip dirs
	if !f.IsDir() {

	}
	fmt.Printf("[+] Analyzed: %v directories and %v files\r",
		countDirs, countFiles)
	return nil
}

// Start the program with the current options. Options param is read only
func Start(options Options) {
	opt = options
	fmt.Println("[+] Starting in directory:", opt.currentDir)
	err := filepath.Walk(opt.currentDir, visit)

	if err != nil {
		fmt.Println("[-]", err)
	}
	fmt.Println()
}
