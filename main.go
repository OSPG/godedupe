package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

const (
	name    string = "godedupe"
	version string = "1.3.1"
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
	cpuprofile         string
	targetDirs         targetDirectories
	fileExt            string
	jsonFile           string
	maxDepth           int
	showCurrentValues  bool
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	enableRecursion    bool
	followSymlinks     bool
	showSummary        bool
	quiet              bool
	showNotification   bool
	sameLine           bool
}

var (
	opt Options
)

// Init the options to run the program
func init() {
	flag.StringVar(&opt.cpuprofile, "cpuprofile", "", "Enable profiling")
	flag.Var(&opt.targetDirs, "t", "Target directories where the program search for duplicated files")
	flag.StringVar(&opt.jsonFile, "json", "", "Export the list of duplicated files to the given json file")
	flag.StringVar(&opt.fileExt, "ext", "", "Only find duplicates for the given extension")
	flag.IntVar(&opt.maxDepth, "d", -1, "Max recursion depth, -1 = no limit. 1 = current directory")
	flag.BoolVar(&opt.excludeEmptyFiles, "z", true, "Exclude the zero length files")
	flag.BoolVar(&opt.excludeHiddenFiles, "e", true, "Exclude the hidden files")
	flag.BoolVar(&opt.showCurrentValues, "debug", false,
		"Show the current values of the program options")
	flag.BoolVar(&opt.enableRecursion, "r", true, "Follow subdirectories (recursion)")
	flag.BoolVar(&opt.followSymlinks, "s", false, "Follow symlinks")
	flag.BoolVar(&opt.showSummary, "m", false, "Show a summary")
	flag.BoolVar(&opt.quiet, "q", false, "Don't show progress info")
	flag.BoolVar(&opt.showNotification, "show-notification", false,
		"Show a desktop notification when the program finish")
	flag.BoolVar(&opt.sameLine, "1", false,
		"Show each set of duplicated files in one line (for scripting). "+
			"It implies -q (quiet) and ignores -m (show summary)")
	flag.Parse()
}

// Header show the program name and current version
func header() {
	fmt.Printf(`------------------------
%s - version %s
------------------------`,
		name, version)
}

// ShowDebugInfo print all the current option values
func showDebugInfo() {
	if opt.showCurrentValues {
		fmt.Printf(`------------------------
Current option values
------------------------
Target directory          : %v
Exclude zero length files : %v
Exclude hidden files      : %v
Ignore symlinks           : %v
Recursive search          : %v
Show a summary            : %v
Quiet                     : %v
Show notification         : %v
File extension            : %v
Max depth                 : %v
Json file                 : %v
Profile output            : %v
Same line                 : %v
------------------------
`,
			opt.targetDirs, opt.excludeEmptyFiles, opt.excludeHiddenFiles,
			opt.followSymlinks, opt.enableRecursion, opt.showSummary,
			opt.quiet, opt.showNotification, opt.fileExt, opt.maxDepth, opt.jsonFile,
			opt.cpuprofile, opt.sameLine)
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
	if opt.sameLine {
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
