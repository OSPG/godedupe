package main

import (
	"fmt"
)

// ReportData contains the basic data to generate a basic report
type ReportData struct {
	duplicates int64
	sets       int
	totalSize  int64
}

// ObtainReportData for this session
func ObtainReportData() ReportData {
	var numDup int64
	var sets int
	var totalSize int64
	for _, v := range DuplicatedFiles {
		dups := len(v.listDuplicated) - 1
		numDup += int64(dups)
		sets++
		for _, f := range v.listDuplicated[1:] {
			totalSize += f.info.Size()
		}
	}
	reportData := ReportData{numDup, sets, totalSize}
	return reportData
}

// ReportDuplicated shows all the information regarding our duplicated files
// if showSummary is true then a summary will printed too
func (report *ReportData) ReportDuplicated(showSummary bool) {
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

	fmt.Printf("[+] %d duplicated files (in %d sets), occupying %v bytes\n",
		report.duplicates, report.sets, ConvertBytes(report.totalSize))
}
