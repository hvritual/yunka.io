package excelx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestExcelFromIOReaderReadsSharedInlineAndSparseRows(t *testing.T) {
	data := workbookFixture(t,
		`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Hello</t></si></sst>`,
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="C1" t="inlineStr"><is><t>World</t></is></c></row><row r="3"><c r="B3"><v>42</v></c></row></sheetData></worksheet>`,
	)
	got := []string{}
	err := ExcelFromIOReader(bytes.NewReader(data), "", func(index int, row []string) error {
		got = append(got, fmt.Sprintf("%d:%q", index, row))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`0:["Hello" "" "World"]`, `1:[]`, `2:["" "42"]`}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
}

func TestExcelReaderRejectsNegativeSharedStringIndex(t *testing.T) {
	data := workbookFixture(t,
		`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>Hello</t></si></sst>`,
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>-1</v></c></row></sheetData></worksheet>`,
	)
	err := ExcelFromIOReader(bytes.NewReader(data), "Sheet1", func(int, []string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "invalid shared string index") {
		t.Fatalf("negative shared-string index escaped validation: %v", err)
	}
}

func TestExcelReaderRejectsRowBeyondWorksheetLimit(t *testing.T) {
	data := workbookFixture(t, "",
		fmt.Sprintf(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="%d"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`, maxExcelRows+1),
	)
	err := ExcelFromIOReader(bytes.NewReader(data), "Sheet1", func(int, []string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "invalid row number") {
		t.Fatalf("oversized row escaped validation: %v", err)
	}
}

func TestExcelReaderRejectsRelationshipTraversal(t *testing.T) {
	data := workbookFixtureWithTarget(t, "../outside.xml", "", `<worksheet/>`)
	err := ExcelFromIOReader(bytes.NewReader(data), "Sheet1", func(int, []string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "escapes xl/") {
		t.Fatalf("relationship traversal escaped validation: %v", err)
	}
}

func workbookFixture(t *testing.T, shared, sheet string) []byte {
	t.Helper()
	return workbookFixtureWithTarget(t, "worksheets/sheet1.xml", shared, sheet)
}

func workbookFixtureWithTarget(t *testing.T, target, shared, sheet string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	add := func(name, body string) {
		t.Helper()
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	add("xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><bookViews><workbookView activeTab="0"/></bookViews><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`)
	add("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="`+target+`"/></Relationships>`)
	if shared != "" {
		add("xl/sharedStrings.xml", shared)
	}
	if target == "worksheets/sheet1.xml" {
		add("xl/worksheets/sheet1.xml", sheet)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
