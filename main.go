package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

const (
	name    string = "godedupe"
	version string = "1.2.1"
)

// Options for start the program
type Options struct {
	showCurrentValues  bool
	cpuprofile         string
	currentDir         string
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	enableRecursion    bool
	ignoreSymLinks     bool
	showSummary        bool
	quiet              bool
	showNotification   bool
	fileExt            string
	maxDepth           int
	jsonFile           string
}

// Init the options to run the program
func initOptions() Options {
	var opt Options
	fmt.Println()
	flag.StringVar(&opt.cpuprofile, "cpuprofile", "", "Enable profiling")
	flag.StringVar(&opt.currentDir, "t", GetUserHome(),
		"Current directory where the program search for duplicated files")
	flag.BoolVar(&opt.excludeEmptyFiles, "z", true, "Exclude the zero length files")
	flag.BoolVar(&opt.excludeHiddenFiles, "h", true, "Exclude the hidden files")
	flag.BoolVar(&opt.showCurrentValues, "debug", false,
		"Show the current values of the program options")
	flag.BoolVar(&opt.enableRecursion, "r", true, "Follow subdirectories (recursion)")
	flag.BoolVar(&opt.ignoreSymLinks, "sym", true, "Ignore symlinks")
	flag.BoolVar(&opt.showSummary, "m", false, "Show a summary")
	flag.BoolVar(&opt.quiet, "q", false, "Don't show progress info")
	flag.BoolVar(&opt.showNotification, "show-notification", false,
		"Show a desktop notification when the program finish")
	flag.StringVar(&opt.fileExt, "ext", "", "Only find duplicates for the given extension")
	flag.IntVar(&opt.maxDepth, "d", -1, "Max recursion depth, -1 = no limit. 1 = current directory")
	flag.StringVar(&opt.jsonFile, "json", "", "Export the list of duplicated files to the given json file")
	flag.Parse()

	return opt
}

// Header show the program name and current version
func header(is_quiet bool) {
	if !is_quiet {
		fmt.Println("------------------------")
		fmt.Printf("%s - version %s\n", name, version)
		fmt.Println("------------------------")
	}
}

// ShowDebugInfo print all the current option values
func showDebugInfo(opt Options) {
	if opt.showCurrentValues && !opt.quiet {
		fmt.Println()
		fmt.Println("------------------------")
		fmt.Println("Current option values")
		fmt.Println("------------------------")
		fmt.Println("Target directory          :", opt.currentDir)
		fmt.Println("Exclude zero length files :", opt.excludeEmptyFiles)
		fmt.Println("Exclude hidden files      :", opt.excludeHiddenFiles)
		fmt.Println("Ignore symlinks           :", opt.ignoreSymLinks)
		fmt.Println("Recursive search          :", opt.enableRecursion)
		fmt.Println("Show a summary            :", opt.showSummary)
		fmt.Println("Quiet                     :", opt.quiet)
		fmt.Println("File extension            :", opt.fileExt)
		fmt.Println("Max depth                 :", opt.maxDepth)
		fmt.Println("Json file                 :", opt.jsonFile)
		fmt.Println("Profile output            :", opt.cpuprofile)
		fmt.Println("------------------------")
	}
}

func trackTime(now time.Time) {
	expired := time.Since(now)
	fmt.Printf("[+] Program terminated in %v\n", expired)
}

func executeCPUProfile(profile string) {
	f, err := os.Create(profile)
	if err != nil {
		panic(err)
	}
	pprof.StartCPUProfile(f)
}

func main() {
	options := initOptions()

	header(options.quiet)

	showDebugInfo(options)

	if options.cpuprofile != "" {
		executeCPUProfile(options.cpuprofile)
		defer pprof.StopCPUProfile()
	}

	if !opt.quiet {
		defer trackTime(time.Now())
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	Start(options)
}
