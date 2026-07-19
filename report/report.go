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

type byteScale struct {
	unit     string
	divisor  float64
	decimals bool
}

func scaleForBytes(bytes int64) byteScale {
	switch {
	case bytes < 1024:
		return byteScale{unit: "bytes", divisor: 1}
	case bytes < 1048576:
		return byteScale{unit: "KB", divisor: 1024, decimals: true}
	case bytes < 1073741824:
		return byteScale{unit: "MB", divisor: 1048576, decimals: true}
	default:
		return byteScale{unit: "GB", divisor: 1073741824, decimals: true}
	}
}

func formatBytes(bytes int64, scale byteScale) string {
	if !scale.decimals {
		return fmt.Sprintf("%d", bytes)
	}

	return fmt.Sprintf("%.2f", float64(bytes)/scale.divisor)
}

// ConvertBytes to convenient convert bytes to other units
func convertBytes(bytes int64) string {
	scale := scaleForBytes(bytes)
	return fmt.Sprintf("%s %s", formatBytes(bytes, scale), scale.unit)
}

func combinedBytes(left int64, right int64) string {
	total := left + right
	scale := scaleForBytes(total)

	return fmt.Sprintf("%s+%s=%s %s",
		formatBytes(left, scale),
		formatBytes(right, scale),
		formatBytes(total, scale),
		scale.unit)
}

func (report *Data) getSummary() string {
	return fmt.Sprintf("%v duplicated files in (%v sets) occupying %v\n",
		report.duplicates, report.sets, convertBytes(report.totalSize))
}

// CombinedSummary returns one summary containing directory and file duplicate totals.
func CombinedSummary(dirReport *DirectoryData, fileReport *Data) string {
	dirDuplicates := int64(0)
	dirSets := 0
	dirSize := int64(0)
	if dirReport != nil {
		dirDuplicates = dirReport.duplicates
		dirSets = dirReport.sets
		dirSize = dirReport.totalSize
	}

	fileDuplicates := int64(0)
	fileSets := 0
	fileSize := int64(0)
	if fileReport != nil {
		fileDuplicates = fileReport.duplicates
		fileSets = fileReport.sets
		fileSize = fileReport.totalSize
	}

	return fmt.Sprintf("%d+%d=%d duplicated entries in (%d+%d=%d sets) occupying %s\n",
		dirDuplicates, fileDuplicates, dirDuplicates+fileDuplicates,
		dirSets, fileSets, dirSets+fileSets,
		combinedBytes(dirSize, fileSize))
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

// DirectoryData contains the data to generate a directory duplicate report.
type DirectoryData struct {
	dupDirs    map[uint64]compare.DuplicatedDirectories
	opt        Opts
	duplicates int64
	sets       int
	totalSize  int64
}

// ObtainDirectoryReportData prepares duplicated directory data for this session.
func ObtainDirectoryReportData(dupDirs map[uint64]compare.DuplicatedDirectories, opts Opts) *DirectoryData {
	var numDup int64
	var sets int
	var totalSize int64
	for _, v := range dupDirs {
		dups := len(v.ListDuplicated) - 1
		numDup += int64(dups)
		sets++
		for _, dir := range v.ListDuplicated[1:] {
			totalSize += dir.Size()
		}
	}
	return &DirectoryData{dupDirs, opts, numDup, sets, totalSize}
}

func (report *DirectoryData) getSummary() string {
	return fmt.Sprintf("%v duplicated directories in (%v sets) occupying %v\n",
		report.duplicates, report.sets, convertBytes(report.totalSize))
}

func (report *DirectoryData) reportDuplicated() {
	wr := bufio.NewWriter(os.Stdout)
	for k, v := range report.dupDirs {
		fmt.Fprintf(wr, "Listing duplicated directories for hash: %x\n\n", k)
		for _, dir := range v.ListDuplicated {
			fmt.Fprintln(wr, dir.Path)
		}
		wr.WriteString("-------------------------\n")
	}

	wr.WriteString("\n")
	wr.Flush()

	if report.opt.ShowSummary {
		fmt.Print(report.getSummary())
	}
}

func (report *DirectoryData) reportSameLine() {
	for k, v := range report.dupDirs {
		fmt.Printf("%x", k)
		for _, dir := range v.ListDuplicated {
			fmt.Printf(" %s", dir.Path)
		}
		fmt.Println()
	}
}

func (report *DirectoryData) exportDuplicate(dstFile string) {
	f, err := os.OpenFile(dstFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer f.Close()

	for k, v := range report.dupDirs {
		var paths []string
		for _, dir := range v.ListDuplicated {
			paths = append(paths, dir.Path)
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

func (report *DirectoryData) showReportNotification() {
	ShowNotification("godedupe finish", report.getSummary())
}

// DoReport does the directory duplicate report.
func (report *DirectoryData) DoReport() {
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
