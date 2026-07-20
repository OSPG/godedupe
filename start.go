package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"godedupe/compare"
	"godedupe/filter"
	"godedupe/report"
)

var (
	countDirs             int
	countFiles            int
	excludePatterns       []string
	parsedExcludePatterns []filter.Pattern
)

func updateDir() {
	countDirs++
}

func updateFile() {
	countFiles++
}

func expandHomeDir(fname string) (string, error) {
	if fname == "~" {
		return os.UserHomeDir()
	}

	if !strings.HasPrefix(fname, "~/") {
		return fname, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, fname[2:]), nil
}

func loadExcludePatterns(fname string) error {
	expandedName, err := expandHomeDir(fname)
	if err != nil {
		return err
	}

	file, err := os.Open(expandedName)
	if err != nil {
		if fname == defaultExcludeFile {
			// only shown a warning, probably the user doesn't want to use it
			fmt.Printf("[?] Exclude patterns will not be used: %s\n", err)
			return nil
		}

		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		excludePatterns = append(excludePatterns, scanner.Text())
	}
	return scanner.Err()
}

func isComparableFile(file compare.File) bool {
	matched := true
	if opt.pattern != "" {
		matched, _ = filepath.Match(opt.pattern, file.Info.Name())
	}

	if !matched {
		return false
	}
	if opt.excludeEmptyFiles && file.Info.Size() == 0 {
		return false
	}
	if !opt.followSymlinks && file.Info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	return true
}

// readDir reads the files from the dir "s" recursively and checks if there are duplicated
func readDir(s string, depth int) *compare.Directory {
	depth++
	info, err := os.Stat(s)
	if err != nil {
		fmt.Printf("[-] Error reading %s: %s\n", s, err)
		return nil
	}
	dir := &compare.Directory{Path: s, Info: info}

	files, err := os.ReadDir(s)
	if err != nil {
		fmt.Printf("[-] Error reading %s: %s\n", s, err)
		return nil
	}
	if len(files) == 0 {
		compare.AddDirectory(dir)
		return dir
	}

	for _, f := range files {
		if f.Name() == ".godedupe_ignore" {
			return nil
		}
	}

	for _, f := range files {

		path := filepath.Join(s, f.Name())

		if opt.excludeHidden && strings.HasPrefix(f.Name(), ".") {
			continue
		}

		matched, err := filter.List(parsedExcludePatterns, path)
		if err != nil {
			fmt.Printf("[-] Error %s\n", err)
			return nil
		}

		if matched {
			continue
		}

		if !f.IsDir() {
			info, err := f.Info()
			if err != nil {
				fmt.Printf("[-] Error reading %s: %s\n", path, err)
				return nil
			}
			updateFile()
			file := compare.File{
				Path: path,
				Info: info,
			}
			if isComparableFile(file) {
				if !opt.directoriesOnly {
					compare.AddFile(file)
				}
				dir.Files = append(dir.Files, file)
			}
		} else if opt.enableRecursion {
			updateDir()
			if depth < opt.maxDepth || opt.maxDepth == -1 {
				childDir := readDir(path, depth)
				if childDir != nil {
					if childDir.Incomplete {
						dir.Incomplete = true
					}
					dir.Children = append(dir.Children, childDir)
				}
			} else {
				dir.Incomplete = true
			}
		} else {
			updateDir()
			dir.Incomplete = true
		}

		if !opt.quiet {
			fmt.Printf("[+] Analyzed: %v directories and %v files\r", countDirs, countFiles)
		}
	}

	compare.AddDirectory(dir)

	return dir
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
	parsedExcludePatterns = filter.ParsePatterns(excludePatterns)

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

	reportOpts := report.Opts{
		JsonFile:         opt.JsonFile,
		ShowSummary:      opt.ShowSummary,
		ShowNotification: opt.ShowNotification,
		SameLine:         opt.SameLine,
	}
	if opt.directoryCache {
		if err := compare.EnableDirectoryHashCache(); err != nil && !opt.quiet {
			fmt.Printf("[?] Directory hash cache disabled: %s\n", err)
		}
		defer func() {
			if err := compare.CloseDirectoryHashCache(); err != nil && !opt.quiet {
				fmt.Printf("[?] Directory hash cache flush failed: %s\n", err)
			}
		}()
	} else {
		_ = compare.CloseDirectoryHashCache()
	}

	compare.ValidateDuplicatedDirectories(!opt.quiet)
	dupFiles := make(map[uint64]compare.Duplicated)
	if !opt.directoriesOnly {
		compare.PruneFileCandidatesCoveredByDirectories(compare.DuplicatedDirectoriesFound)
		compare.ValidateDuplicatedFiles(!opt.quiet)
		dupFiles = compare.FilterFilesCoveredByDirectories(compare.DuplicatedFiles, compare.DuplicatedDirectoriesFound, !opt.quiet)
	}

	displayOpts := reportOpts
	displayOpts.ShowSummary = false

	dirReport := report.ObtainDirectoryReportData(compare.DuplicatedDirectoriesFound, displayOpts)
	fileReport := report.ObtainReportData(dupFiles, displayOpts)

	reported := false
	if len(compare.DuplicatedDirectoriesFound) > 0 {
		dirReport.DoReport()
		reported = true
	}

	if !opt.directoriesOnly && (len(dupFiles) > 0 || !reported) {
		fileReport.DoReport()
	}

	if reportOpts.ShowSummary {
		fmt.Print(report.CombinedSummary(dirReport, fileReport))
	}
}
