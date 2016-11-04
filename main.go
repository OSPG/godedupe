package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
)

const (
	name    string = "godedupe"
	version string = "0.0.1"
)

var (
	cpuprofile string

	currentDir         string
	excludeEmptyDir    bool
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	showCurrentValues  bool
	enableRecursion    bool
	ignoreSymLinks     bool
	showSummary        bool
	quiet              bool
)

// Options for start the program
type Options struct {
	currentDir         string
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	enableRecursion    bool
	ignoreSymLinks     bool
	showSummary        bool
	quiet              bool
}

// Init the options to run the program
func Init() {
	flag.StringVar(&cpuprofile, "cpuprofile", "", "Enable profiling")

	flag.StringVar(&currentDir, "t", GetUserHome(),
		"Current directory where the program search for duplicated files")
	flag.BoolVar(&excludeEmptyFiles, "z", true, "Exclude the zero length files")
	flag.BoolVar(&excludeHiddenFiles, "h", true, "Exclude the hidden files")
	flag.BoolVar(&showCurrentValues, "debug", false,
		"Show the current values of the program options")
	flag.BoolVar(&enableRecursion, "r", true, "Follow subdirectories (recursion)")
	flag.BoolVar(&ignoreSymLinks, "sym", true, "Ignore symlinks")
	flag.BoolVar(&showSummary, "m", false, "Show a summary")
	flag.BoolVar(&quiet, "q", false, "Don't show status info")
	flag.Parse()
}

// Header show the program name and current version
func Header() {
	fmt.Println("------------------------")
	fmt.Printf("%s - version %s\n", name, version)
	fmt.Println("------------------------")
}

// ShowDebugInfo print all the current option values
func ShowDebugInfo() {
	if showCurrentValues {
		fmt.Println()
		fmt.Println("------------------------")
		fmt.Println("Current option values")
		fmt.Println("------------------------")
		fmt.Println("Target directory          :", currentDir)
		fmt.Println("Exclude zero length files :", excludeEmptyFiles)
		fmt.Println("Exclude hidden files      :", excludeHiddenFiles)
		fmt.Println("Ignore symlinks           :", ignoreSymLinks)
		fmt.Println("Recursive search          :", enableRecursion)
		fmt.Println("Show a summary            :", showSummary)
		fmt.Println("Quiet                     :", quiet)
		if cpuprofile != "" {
			fmt.Println("Profile output            :", cpuprofile)
		}
		fmt.Println("------------------------")
	}
}

func main() {
	Header()
	Init()
	ShowDebugInfo()

	options := Options{
		currentDir,
		excludeEmptyFiles,
		excludeHiddenFiles,
		enableRecursion,
		ignoreSymLinks,
		showSummary,
		quiet,
	}

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	Start(options)
}
