package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/OSPG/godedupe/report"
)

const (
	name    string = "godedupe"
	version string = "1.4.0"
)

type targetDirectories []string

func (i *targetDirectories) String() string {
	return ""
}

func (i *targetDirectories) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// Options for start the program
type Options struct {
	report.Opts

	cpuprofile         string
	targetDirs         targetDirectories
	pattern            string
	maxDepth           int
	showCurrentValues  bool
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	enableRecursion    bool
	followSymlinks     bool
	quiet              bool
}

var (
	opt Options
)

// Init the options to run the program
func init() {
	flag.StringVar(&opt.cpuprofile, "cpuprofile", "", "Enable profiling")
	flag.Var(&opt.targetDirs, "t", "Target directories where the program search for duplicated files")
	flag.StringVar(&opt.JsonFile, "json", "", "Export the list of duplicated files to the given json file")
	flag.StringVar(&opt.pattern, "pattern", "", "Only find duplicates if the given pattern match")
	flag.IntVar(&opt.maxDepth, "d", -1, "Max recursion depth, -1 = no limit. 1 = current directory")
	flag.BoolVar(&opt.excludeEmptyFiles, "z", true, "Exclude the zero length files")
	flag.BoolVar(&opt.excludeHiddenFiles, "e", true, "Exclude the hidden files")
	flag.BoolVar(&opt.showCurrentValues, "debug", false,
		"Show the current values of the program options")
	flag.BoolVar(&opt.enableRecursion, "r", true, "Follow subdirectories (recursion)")
	flag.BoolVar(&opt.followSymlinks, "s", false, "Follow symlinks")
	flag.BoolVar(&opt.ShowSummary, "m", false, "Show a summary")
	flag.BoolVar(&opt.quiet, "q", false, "Don't show progress info")
	flag.BoolVar(&opt.ShowNotification, "show-notification", false,
		"Show a desktop notification when the program finish")
	flag.BoolVar(&opt.SameLine, "1", false, "Show each set of duplicated files in one line."+
		"It implies -q (quiet) and ignores -m (show summary)")
	flag.Parse()
}

// Header show the program name and current version
func header() {
	fmt.Println("------------------------")
	fmt.Printf("%s - version %s\n", name, version)
	fmt.Println("------------------------")
}

// ShowDebugInfo print all the current option values
func showDebugInfo() {
	if opt.showCurrentValues {
		fmt.Println()
		fmt.Println("------------------------")
		fmt.Println("Current option values")
		fmt.Println("------------------------")
		fmt.Println("Target directory          :", opt.targetDirs)
		fmt.Println("Exclude zero length files :", opt.excludeEmptyFiles)
		fmt.Println("Exclude hidden files      :", opt.excludeHiddenFiles)
		fmt.Println("Ignore symlinks           :", opt.followSymlinks)
		fmt.Println("Recursive search          :", opt.enableRecursion)
		fmt.Println("Show a summary            :", opt.ShowSummary)
		fmt.Println("Quiet                     :", opt.quiet)
		fmt.Println("Show notification         :", opt.ShowNotification)
		fmt.Println("Pattern                   :", opt.pattern)
		fmt.Println("Max depth                 :", opt.maxDepth)
		fmt.Println("Json file                 :", opt.JsonFile)
		fmt.Println("Profile output            :", opt.cpuprofile)
		fmt.Println("Same line                 :", opt.SameLine)
		fmt.Println("------------------------")
	}
}

func trackTime(now time.Time) {
	fmt.Printf("[+] Program terminated in %v\n", time.Since(now))
}

func executeCPUProfile(profile string) {
	f, err := os.Create(profile)
	if err != nil {
		panic(err)
	}
	pprof.StartCPUProfile(f)
}

func main() {
	if opt.SameLine {
		opt.quiet = true
	}
	if !opt.quiet {
		header()
	}
	showDebugInfo()
	if opt.cpuprofile != "" {
		executeCPUProfile(opt.cpuprofile)
		defer pprof.StopCPUProfile()
	}
	st := time.Now()
	start()
	trackTime(st)
}
