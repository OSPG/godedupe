package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	update(file.info)

	// Only scan for files of a given format
	if !file.info.IsDir() && opt.fileExt != "" && !strings.HasSuffix(file.info.Name(), opt.fileExt) {
		return false
	}

	if opt.excludeEmptyFiles && file.info.Size() == 0 {
		return false
	}

	if opt.excludeHiddenFiles && strings.HasPrefix(file.info.Name(), ".") {
		// hidden file or directory
		return false
	}

	if opt.ignoreSymLinks && file.info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	// only make hash for files
	if !file.info.IsDir() {
		AddFile(file)
	}
	if !opt.quiet {
		fmt.Printf("[+] Analyzed: %v directories and %v files\r",
			countDirs, countFiles)
	}
	return true
}

// readDir reads the files from the dir "s" recursively and checks if there are duplicated
func readDir(s string, depth int) error {
	depth++

	files, err := ioutil.ReadDir(s)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}

	for _, f := range files {
		if f.Name() == ".godedupe_ignore" {
			return nil
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
			} else if opt.ignoreSymLinks && file.info.Mode()&os.ModeSymlink != 0 {
			} else {
				AddFile(file)
			}
		} else if opt.enableRecursion {
			if depth < opt.maxDepth || opt.maxDepth == -1 {
				readDir(path, depth)
			}
		}
	}
	return nil
}

// Start the program with the current options. Options param is read only
func Start(options Options) {
	opt = options

	if info, err := os.Stat(opt.currentDir); err == nil && !info.IsDir() && !opt.quiet {
		fmt.Printf("[-] %s is not a valid directory", info.Name())
		return
	}
	if !opt.quiet {
		fmt.Println("[+] Starting in directory:", opt.currentDir)
	}

	err := readDir(opt.currentDir, 0)
	if err != nil && !opt.quiet {
		fmt.Println("[-]", err)
	}

	if !opt.quiet {
		fmt.Printf("\n[+] Stage 1 / 3 completed\n")
	}

	ValidateDuplicatedFiles()

	reportData := ObtainReportData()
	reportData.ReportDuplicated(opt.showSummary)

	file, err := os.Open("icon/success.png")
	if err != nil {
		if !opt.quiet {
			fmt.Println("[-]", err)
		}
		return
	}
	absDir, err := filepath.Abs(file.Name())
	if err != nil {
		if !opt.quiet {
			fmt.Println("[-]", err)
		}
		return
	}
	summary := fmt.Sprintf("%v duplicated files in (%v sets) occupying %v bytes\n",
		reportData.duplicates, reportData.sets, ConvertBytes(reportData.totalSize))
	notification := Notification{"godedupe finish", summary, absDir}
	notification.ShowNotification(opt.showNotification)
}
