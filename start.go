package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var opt Options

func visit(path string, f os.FileInfo, err error) error {
	if err != nil {
		log.Println(err)
	}

	if opt.excludeEmptyFiles && f.Size() == 0 {
		return nil
	}
	fmt.Printf("Visited: %s\n", path)
	return nil
}

// Start the program with the current options. Options param is read only
func Start(options Options) {
	opt = options

	dir := opt.currentDir
	fmt.Println("Starting in directory:", dir)
	err := filepath.Walk(dir, visit)

	if err != nil {
		log.Println(err)
	}
}
