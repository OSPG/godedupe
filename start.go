package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	countDirs  int
	countFiles int
	opt        Options
)

var mutx sync.Mutex

func update(f os.FileInfo) {
	if f.IsDir() {
		countDirs++
	} else {
		countFiles++
	}
}

// readDir reads the files from the dir "s" recursively and checks if there are duplicated
func readDir(s string, depth int) {
	depth++

	files, err := ioutil.ReadDir(s)
	if err != nil {
		fmt.Printf("[-] Directory %s could not be read and will be ignored. Error %s\n", err)
		return
	}
	if len(files) == 0 {
		return
	}

	for _, f := range files {
		if f.Name() == ".godedupe_ignore" {
			return
		}
	}

	for _, f := range files {

		path := s + string(filepath.Separator) + f.Name()
		file := File{
			path,
			f,
		}

		update(file.info)

		if !opt.quiet {
			fmt.Printf("[+] Analyzed: %v directories and %v files\r", countDirs, countFiles)
		}

		if !file.info.IsDir() {
			// Only scan for files of a given extension
			if opt.fileExt != "" && !strings.HasSuffix(file.info.Name(), opt.fileExt) {
			} else if opt.excludeEmptyFiles && file.info.Size() == 0 {
			} else if opt.excludeHiddenFiles && strings.HasPrefix(file.info.Name(), ".") {
			} else if !opt.followSymlinks && file.info.Mode()&os.ModeSymlink != 0 {
			} else {
				mutx.Lock()
				AddFile(file)
				mutx.Unlock()
			}
		} else if opt.enableRecursion {
			if depth < opt.maxDepth || opt.maxDepth == -1 {
				readDir(path, depth)
			}
		}
	}
}

// Start the program with the targetDirs options. Options param is read only
func Start(options Options) {
	// Set the global variable so readDir function can access to the options
	opt = options

	if len(opt.targetDirs) == 0 {
		fmt.Println("Errorr: No directory found")
		return
	}

	for _, dir := range opt.targetDirs {
		if info, err := os.Stat(dir); err == nil && !info.IsDir() && !opt.quiet {
			fmt.Printf("[-] %s is not a valid directory", info.Name())
			return
		}
	}

	for _, dir := range opt.targetDirs {
		if !opt.quiet {
			fmt.Println("[+] Reading directory:", dir)
		}
		readDir(dir, 0)
	}

	if !opt.quiet {
		fmt.Printf("\n[+] Stage 1 / 3 completed\n")
	}

	ValidateDuplicatedFiles()

	data := ObtainReportData(opt)
	data.DoReport()
}
