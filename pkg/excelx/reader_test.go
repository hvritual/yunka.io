package excelx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"codeberg.org/tealeg/xlsx/v4"
)

func TestExcelReaderPreservesActiveSheetAndSparseRows(t *testing.T) {
	contents := testWorkbook(t)

	var selected [][]string
	if err := ExcelFromIOReader(bytes.NewReader(contents), "", func(_ int, row []string) error {
		selected = append(selected, append([]string(nil), row...))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if want := [][]string{{"selected"}}; !reflect.DeepEqual(selected, want) {
		t.Fatalf("active sheet rows=%#v want=%#v", selected, want)
	}

	type observedRow struct {
		Index int
		Row   []string
	}
	var observed []observedRow
	if err := ExcelFromIOReader(bytes.NewReader(contents), "First", func(index int, row []string) error {
		observed = append(observed, observedRow{Index: index, Row: append([]string(nil), row...)})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []observedRow{
		{Index: 0, Row: []string{"first", "", "third"}},
		{Index: 1, Row: nil},
		{Index: 2, Row: []string{"1234.50"}},
	}
	if !reflect.DeepEqual(observed, want) {
		t.Fatalf("explicit sheet rows=%#v want=%#v", observed, want)
	}
}

func TestExcelFromFileAndCallbackError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.xlsx")
	if err := os.WriteFile(path, testWorkbook(t), 0o600); err != nil {
		t.Fatal(err)
	}
	stop := errors.New("stop")
	err := ExcelFromFile(path, "First", func(index int, row []string) error {
		if index == 0 && len(row) > 0 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) {
		t.Fatalf("callback error=%v want=%v", err, stop)
	}
}

func TestExcelReaderRejectsMissingSheetAndInvalidInputs(t *testing.T) {
	contents := testWorkbook(t)
	if err := ExcelFromIOReader(bytes.NewReader(contents), "missing", func(int, []string) error { return nil }); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing sheet error=%v", err)
	}
	if err := ExcelFromIOReader(nil, "", func(int, []string) error { return nil }); err == nil {
		t.Fatal("nil reader was accepted")
	}
	if err := ExcelFromIOReader(bytes.NewReader(contents), "", nil); err == nil {
		t.Fatal("nil callback was accepted")
	}
}

func testWorkbook(t *testing.T) []byte {
	t.Helper()
	file := xlsx.NewFile()
	first, err := file.AddSheet("First")
	if err != nil {
		t.Fatal(err)
	}
	row := first.AddRow()
	row.AddCell().SetString("first")
	row.AddCell()
	row.AddCell().SetString("third")
	first.AddRow()
	row = first.AddRow()
	row.AddCell().SetFloatWithFormat(1234.5, "0.00")

	selected, err := file.AddSheet("Selected")
	if err != nil {
		t.Fatal(err)
	}
	selected.AddRow().AddCell().SetString("selected")

	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	return setWorkbookActiveTab(t, buffer.Bytes(), 1)
}

func setWorkbookActiveTab(t *testing.T, contents []byte, active int) []byte {
	t.Helper()
	source, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	target := zip.NewWriter(&output)
	found := false
	activeAttribute := fmt.Sprintf(`activeTab="%d"`, active)
	activePattern := regexp.MustCompile(`activeTab="[^"]*"`)
	for _, entry := range source.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			reader.Close()
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		if entry.Name == "xl/workbook.xml" {
			found = true
			text := string(data)
			start := strings.Index(text, "<workbookView")
			if start < 0 {
				t.Fatal("workbookView is missing")
			}
			endRelative := strings.Index(text[start:], ">")
			if endRelative < 0 {
				t.Fatal("workbookView start tag is malformed")
			}
			end := start + endRelative + 1
			tag := text[start:end]
			if activePattern.MatchString(tag) {
				tag = activePattern.ReplaceAllString(tag, activeAttribute)
			} else {
				tag = strings.Replace(tag, "<workbookView", "<workbookView "+activeAttribute, 1)
			}
			data = []byte(text[:start] + tag + text[end:])
		}
		header := entry.FileHeader
		writer, err := target.CreateHeader(&header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if !found {
		t.Fatal("workbook.xml is missing")
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
