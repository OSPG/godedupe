package main

import (
	"fmt"
	"os"
	"path/filepath"
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
	if opt.excludeEmptyFiles && f.Size() == 0 {
		return nil
	}
	update(f)

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
