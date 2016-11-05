package main

import (
	"fmt"
)

// ReportDuplicated shows all the information regarding our duplicated files
// if showSummary is true then a summary will printed too
func ReportDuplicated(showSummary bool) {
	if opt.quiet {
		return
	}
	//fmt.Printf("\n\nLISTING DUPLICATED FILES\n")
	//fmt.Printf("-------------------------\n")

	//for k, v := range DuplicatedFiles {
	//	fmt.Printf("Listing duplicateds for hash : %x\n\n", k)
	//	for _, f := range v.listDuplicated {
	//		fmt.Println(f.path)
	//	}
	//	fmt.Printf("-------------------------\n")
	//}

	//fmt.Println("END OF LIST")
	//fmt.Println()

	if showSummary {
		numDup := 0
		sets := 0
		totalSize := int64(0)
		for _, v := range DuplicatedFiles {
			dups := len(v.listDuplicated) - 1
			numDup += dups
			sets++
			for _, f := range v.listDuplicated[1:] {
				totalSize += f.info.Size()
			}
		}
		fmt.Printf("[+] %d duplicated files (in %d sets), occupying %d bytes\n",
			numDup, sets, totalSize)
	}
}
