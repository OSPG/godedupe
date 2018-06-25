package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/OSPG/godedupe/compare"
	"github.com/OSPG/godedupe/report"
)

var (
	countDirs  int
	countFiles int
	regexRule  *regexp.Regexp
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

		if !file.Info.IsDir() {
			// Only scan for files of a given extension
			if opt.regex != "" && !regexRule.MatchString(file.Info.Name()) {
			} else if opt.excludeEmptyFiles && file.Info.Size() == 0 {
			} else if opt.excludeHiddenFiles && strings.HasPrefix(file.Info.Name(), ".") {
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

		if opt.regex != "" {
			r, err := regexp.Compile(opt.regex)
			regexRule = r
			if err != nil {
				fmt.Println("[-] Could not compile regular expression: ", err)
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
