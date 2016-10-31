package main

import (
	"flag"
	"fmt"
)

const name string = "godedupe"
const version string = "0.0.1"

var currentDir string
var excludeEmptyDir bool
var excludeEmptyFiles bool
var excludeHiddenFiles bool
var showCurrentValues bool
var ignoreSymLinks bool

// Init the options to run the program
func Init() {
	flag.StringVar(&currentDir, "d", GetUserHome(),
		"Current directory where the program search for duplicated files")
	flag.BoolVar(&excludeEmptyDir, "e", false, "Exclude the empty directories")
	flag.BoolVar(&excludeEmptyFiles, "z", false, "Exclude the zero length files")
	flag.BoolVar(&excludeHiddenFiles, "h", false, "Exclude the hidden files")
	flag.BoolVar(&showCurrentValues, "debug", false,
		"Show the current values of the program options")
	flag.BoolVar(&ignoreSymLinks, "sym", true, "Ignore symlinks")
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
		fmt.Println("Directory                 :", currentDir)
		fmt.Println("Exclude empty dirs        :", excludeEmptyDir)
		fmt.Println("Exclude zero length files :", excludeEmptyFiles)
		fmt.Println("Exclude hidden files      :", excludeHiddenFiles)
		fmt.Println("Ignore symlinks           :", ignoreSymLinks)
		fmt.Println("------------------------")
	}
}

// Options for start the program
type Options struct {
	currentDir         string
	excludeEmptyDir    bool
	excludeEmptyFiles  bool
	excludeHiddenFiles bool
	ignoreSymLinks     bool
}

func main() {
	Header()
	Init()
	ShowDebugInfo()

	options := Options{
		currentDir,
		excludeEmptyDir,
		excludeEmptyFiles,
		excludeHiddenFiles,
		ignoreSymLinks,
	}

	Start(options)
}
