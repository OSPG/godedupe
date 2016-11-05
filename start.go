package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
)

var (
	countDirs  int
	countFiles int
	opt        Options
)

func update(f os.FileInfo) {
	if f.IsDir() {
		countDirs++
	} else {
		countFiles++
	}
}

// checkFile checks if we have to add this file.
// Returns true if we have to recurse or false if we don't
func checkFile(file File) bool {
	if opt.excludeEmptyFiles && !file.info.IsDir() && file.info.Size() == 0 {
		return false
	}
	if opt.excludeHiddenFiles && strings.HasPrefix(file.info.Name(), ".") {
		// hidden file or directory
		return false
	}
	if opt.ignoreSymLinks && file.info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	update(file.info)

	// only make hash for files, skip dirs
	if !file.info.IsDir() {
		ComparePartialFile(file)
	}
	if !opt.quiet {
		fmt.Printf("[+] Analyzed: %v directories and %v files\r",
			countDirs, countFiles)
	}
	return true
}

// readDir reads the files from the dir "s" recursively and checks if there are duplicated
func readDir(s string) error {
	files, err := ioutil.ReadDir(s)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	for _, f := range files {
		path := s + "/" + f.Name()
		file := File{
			path,
			f,
		}

		recurse := checkFile(file)

		if opt.enableRecursion && recurse && file.info.IsDir() {
			readDir(path)
		}
	}
	return nil
}

// reportDuplicated shows all the information regarding our duplicated files
// if showSummary is true then a summary will printed too
func reportDuplicated(showSummary bool) {
	fmt.Printf("\n\nLISTING DUPLICATED FILES\n")
	fmt.Printf("-------------------------\n")

	for k, v := range DuplicatedFiles {
		fmt.Printf("Listing duplicateds for hash : %x\n\n", k)
		for _, f := range v.listDuplicated {
			fmt.Println(f.path)
		}
		fmt.Printf("-------------------------\n")
	}

	fmt.Println("END OF LIST")
	fmt.Println()

	if showSummary {
		numDup := 0
		sets := 0
		totalSize := int64(0)
		for _, v := range DuplicatedFiles {
			dups := len(v.listDuplicated) - 1
			numDup += dups
			sets++
			for _, f := range v.listDuplicated[1:] {
				totalSize += f.info.Size()
			}
		}
		fmt.Printf("%d duplicated files (in %d sets), occupying %d bytes",
			numDup, sets, totalSize)
	}
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

	fmt.Printf("\nPartial search done. Deleting false positives\n")

	ValidateDuplicatedFiles()

	reportDuplicated(opt.showSummary)

	fmt.Println()
}
