package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"godedupe/compare"
)

type Opts struct {
	JsonFile         string
	ShowSummary      bool
	ShowNotification bool
	SameLine         bool
}

// Data contains the basic data to generate a basic report
type Data struct {
	dupFiles   map[uint64]compare.Duplicated
	opt        Opts
	duplicates int64
	sets       int
	totalSize  int64
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

func (report *Data) getSummary() string {
	return fmt.Sprintf("%v duplicated files in (%v sets) occupying %v\n",
		report.duplicates, report.sets, convertBytes(report.totalSize))
}

// ObtainReportData for this session
func ObtainReportData(dupFiles map[uint64]compare.Duplicated, opts Opts) *Data {

	var numDup int64
	var sets int
	var totalSize int64
	for _, v := range dupFiles {
		dups := len(v.ListDuplicated) - 1
		numDup += int64(dups)
		sets++
		for _, f := range v.ListDuplicated[1:] {
			totalSize += f.Info.Size()
		}
	}
	return &Data{dupFiles, opts, numDup, sets, totalSize}
}

// reportDuplicated shows all the information regarding our duplicated files
func (report *Data) reportDuplicated() {
	wr := bufio.NewWriter(os.Stdout)
	for k, v := range report.dupFiles {
		fmt.Fprintf(wr, "Listing duplicateds for hash: %x\n\n", k)
		for _, f := range v.ListDuplicated {
			fmt.Fprintln(wr, f.Path)
		}
		wr.WriteString("-------------------------\n")
	}

	wr.WriteString("\n")
	wr.Flush()

	if report.opt.ShowSummary {
		fmt.Print(report.getSummary())
	}
}

func (report *Data) reportSameLine() {
	for k, v := range report.dupFiles {
		fmt.Printf("%x", k)
		for _, f := range v.ListDuplicated {
			fmt.Printf(" %s", f.Path)
		}
		fmt.Println()
	}
}

// ExportDuplicate exports the list of duplicated files to the given file
func (report *Data) exportDuplicate(dstFile string) {
	f, err := os.OpenFile(dstFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer f.Close()

	for k, v := range report.dupFiles {
		var paths []string
		for _, f := range v.ListDuplicated {
			paths = append(paths, f.Path)
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

func (report *Data) showReportNotification() {
	ShowNotification("godedupe finish", report.getSummary())
}

// DoReport does the report, printing it to stdout, and exporting it to a file
// or showing a notification if necessary
func (report *Data) DoReport() {
	if report.opt.SameLine {
		report.reportSameLine()
	} else {
		report.reportDuplicated()
	}

	if report.opt.JsonFile != "" {
		report.exportDuplicate(report.opt.JsonFile)
	}

	if report.opt.ShowNotification {
		report.showReportNotification()
	}
}
