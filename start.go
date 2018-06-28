package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/OSPG/godedupe/compare"
	"github.com/OSPG/godedupe/report"
)

var (
	countDirs  int
	countFiles int
)

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
		fmt.Printf("[-] Error reading %s: %s\n", s, err)
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
		path := filepath.Join(s, f.Name())
		file := compare.File{
			path,
			f,
		}

		update(file.Info)

		if !opt.quiet {
			fmt.Printf("[+] Analyzed: %v directories and %v files\r", countDirs, countFiles)
		}

		if opt.excludeHiddenFiles && strings.HasPrefix(file.Info.Name(), ".") {
			return
		}

		if !file.Info.IsDir() {
			// Only scan for files of a given extension
			matched := true
			if opt.pattern != "" {
				matched, _ = filepath.Match(opt.pattern, file.Info.Name())
			}
			if !matched {
			} else if opt.excludeEmptyFiles && file.Info.Size() == 0 {
			} else if !opt.followSymlinks && file.Info.Mode()&os.ModeSymlink != 0 {
			} else {
				compare.AddFile(file)
			}
		} else if opt.enableRecursion {
			if depth < opt.maxDepth || opt.maxDepth == -1 {
				readDir(path, depth)
			}
		}
	}
}

// Start the program with the targetDirs options. Options param is read only
func start() {
	// Set the global variable so readDir function can access to the options
	if len(opt.targetDirs) == 0 {
		fmt.Println("error: directory must be specified. See help.")
		return
	}

	for _, dir := range opt.targetDirs {
		if info, err := os.Stat(dir); err == nil && !info.IsDir() && !opt.quiet {
			// This should return an error to avoid hiding potential configuration errors
			fmt.Printf("[-] %s is not a valid directory", info.Name())
			return
		}
	}
	for _, dir := range opt.targetDirs {
		if !opt.quiet {
			fmt.Println("[+] Reading directory:", dir)
		}

		if opt.pattern != "" {
			_, err := filepath.Match(opt.pattern, "")
			if err != nil {
				fmt.Println("[-] Bad Ppattern: ", err)
				return
			}
		}
		readDir(dir, 0)
	}
	if !opt.quiet {
		fmt.Printf("\n[+] Stage 1 / 3 completed\n")
	}
	compare.ValidateDuplicatedFiles(!opt.quiet)
	reportOpts := report.Opts{opt.JsonFile, opt.ShowSummary, opt.ShowNotification, opt.SameLine}
	report.ObtainReportData(compare.DuplicatedFiles, reportOpts).DoReport()
}
