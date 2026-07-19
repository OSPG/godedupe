package report

import "testing"

func TestConvertBytesUsesExpectedUnitsAtBoundaries(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{bytes: 1023, want: "1023 bytes"},
		{bytes: 1024, want: "1.00 KB"},
		{bytes: 1048576, want: "1.00 MB"},
	}

	for _, tt := range tests {
		if got := convertBytes(tt.bytes); got != tt.want {
			t.Fatalf("convertBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestCombinedSummaryUsesTotalUnitForOccupiedSize(t *testing.T) {
	dirReport := &DirectoryData{
		duplicates: 1,
		sets:       1,
		totalSize:  16 * 1024,
	}
	fileReport := &Data{
		duplicates: 2,
		sets:       1,
		totalSize:  4 * 1024 * 1024,
	}

	got := CombinedSummary(dirReport, fileReport)
	want := "1+2=3 duplicated entries in (1+1=2 sets) occupying 0.02+4.00=4.02 MB\n"
	if got != want {
		t.Fatalf("CombinedSummary() = %q, want %q", got, want)
	}
}
