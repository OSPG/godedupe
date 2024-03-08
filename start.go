package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/OSPG/godedupe/compare"
	"github.com/OSPG/godedupe/report"
	"godedupe/filter"
)

var (
	countDirs       int
	countFiles      int
	excludePatterns []string
)

func update(f os.FileInfo) {
	if f.IsDir() {
		countDirs++
	} else {
		countFiles++
	}
}

func loadExcludePatterns(fname string) error {
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		excludePatterns = append(excludePatterns, scanner.Text())
	}
	return scanner.Err()
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

		if opt.excludeHiddenFiles && strings.HasPrefix(file.Info.Name(), ".") {
			continue
		}

		p := filter.ParsePatterns(excludePatterns)
		matched, err := filter.List(p, path)
		if err != nil {
			fmt.Printf("[-] Error %s\n", err)
			return
		}

		if matched {
			continue
		}

		update(file.Info)

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

		if !opt.quiet {
			fmt.Printf("[+] Analyzed: %v directories and %v files\r", countDirs, countFiles)
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

	err := loadExcludePatterns(opt.excludeFrom)
	if err != nil {
		fmt.Printf("[-] Error reading %s: %s\n", opt.excludeFrom, err)
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
