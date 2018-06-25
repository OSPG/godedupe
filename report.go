package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// ReportData contains the basic data to generate a basic report
type ReportData struct {
	duplicates int64
	sets       int
	totalSize  int64
	opt        Options
}

type jsonExport struct {
	Hash  uint64
	Paths []string
}

// ConvertBytes to convenient convert bytes to other units
func convertBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%v bytes", bytes)
	} else if bytes > 1024 && bytes < 1048576 {
		return fmt.Sprintf("%.2f KB", float32(bytes)/float32(1024))
	} else if bytes > 1048576 && bytes < 1073741824 {
		return fmt.Sprintf("%.2f MB", float32(bytes)/float32(1048576))
	}
	return fmt.Sprintf("%.2f GB", float32(bytes)/float32(1073741824))
}

func (report *ReportData) getSummary() string {
	return fmt.Sprintf("%v duplicated files in (%v sets) occupying %v bytes\n",
		report.duplicates, report.sets, convertBytes(report.totalSize))
}

// ObtainReportData for this session
func ObtainReportData() *ReportData {
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
	return &ReportData{numDup, sets, totalSize, opt}
}

// reportDuplicated shows all the information regarding our duplicated files
func (report *ReportData) reportDuplicated() {
	wr := bufio.NewWriter(os.Stdout)
	wr.WriteString("LISTING DUPLICATED FILES")
	wr.WriteString("-------------------------")

	for k, v := range DuplicatedFiles {
		fmt.Fprintf(wr, "Listing duplicateds for hash: %x\n\n", k)
		for _, f := range v.listDuplicated {
			fmt.Fprintln(wr, f.path)
		}
		wr.WriteString("-------------------------\n")
	}
	wr.Flush()
	if report.opt.showSummary {
		fmt.Print(report.getSummary())
	}
}

func (report *ReportData) reportSameLine() {
	for k, v := range DuplicatedFiles {
		fmt.Printf("%x", k)
		for _, f := range v.listDuplicated {
			fmt.Printf(" %s", f.path)
		}
		fmt.Println()
	}
}

// ExportDuplicate exports the list of duplicated files to the given file
func (report *ReportData) exportDuplicate(dstFile string) {
	f, err := os.OpenFile(dstFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
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

		jsonData := &jsonExport{Hash: k, Paths: paths}
		json, err := json.MarshalIndent(jsonData, "", "\t")
		if err != nil {
			fmt.Println(err)
		}

		if _, err = f.Write(json); err != nil {
			fmt.Println(err)
			return
		}
	}
}

func (report *ReportData) showReportNotification() {
	ShowNotification("godedupe finish", report.getSummary())
}

// DoReport does the report, printing it to stdout, and exporting it to a file
// or showing a notification if necessary
func (report *ReportData) DoReport() {
	if report.opt.sameLine {
		report.reportSameLine()
	} else {
		report.reportDuplicated()
	}

	if report.opt.jsonFile != "" {
		report.exportDuplicate(report.opt.jsonFile)
	}

	if report.opt.showNotification {
		report.showReportNotification()
	}
}
