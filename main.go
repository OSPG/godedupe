package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	skipVoid bool
}

var opt Options

func visit(path string, f os.FileInfo, err error) error {
	if err != nil {
		panic(err)
	}

	if opt.skipVoid && f.Size() == 0 {
		return nil
	}

	fmt.Printf("Visited: %s\n", path)
	return nil
}

func main() {
	opt = Options{}
	//TODO: Get options from command line flags

	err := filepath.Walk(GetUserHome(), visit)

	if err != nil {
		panic(err)
	}
}
