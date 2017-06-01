package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReportData contains the basic data to generate a basic report
type ReportData struct {
	duplicates int64
	sets       int
	totalSize  int64
}

type JsonExport struct {
	Hash  uint64
	Paths []string
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
	fmt.Printf("LISTING DUPLICATED FILES\n")
	fmt.Printf("-------------------------\n")

	for k, v := range DuplicatedFiles {
		fmt.Printf("Listing duplicateds for hash : %x\n\n", k)
		for _, f := range v.listDuplicated {
			fmt.Println(f.path)
		}
		fmt.Printf("-------------------------\n")
	}

	fmt.Println("END OF LIST")
	fmt.Println()

	if showSummary {
		fmt.Printf("[+] %d duplicated files (in %d sets), occupying %v\n",
			report.duplicates, report.sets, ConvertBytes(report.totalSize))
	}
}

func (report *ReportData) ReportSameLine() {
	for k, v := range DuplicatedFiles {
		fmt.Printf("%x", k)
		for _, f := range v.listDuplicated {
			fmt.Printf(" %s", f.path)
		}
		fmt.Println()
	}
}

// ExportDuplicate exports the list of duplicated files to the given file
func (report *ReportData) ExportDuplicate(dst_file string) {
	f, err := os.OpenFile(dst_file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer f.Close()

	for k, v := range DuplicatedFiles {
		var paths []string
		for _, f := range v.listDuplicated {
			paths = append(paths, f.path)
		}

		json_data := &JsonExport{Hash: k, Paths: paths}
		json, err := json.MarshalIndent(json_data, "", "\t")
		if err != nil {
			fmt.Println(err)
		}

		if _, err = f.Write(json); err != nil {
			fmt.Println(err)
			return
		}
	}
}
