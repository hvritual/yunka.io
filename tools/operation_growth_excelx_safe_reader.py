#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

reader = r'''package excelx

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
)

const (
	maxWorkbookPartSize = 16 << 20
	maxSharedStringsSize = 256 << 20
	maxWorksheetPartSize = 512 << 20
	maxExcelRows         = 1048576
	maxExcelColumns      = 16384
)

func ExcelFromIOReader(reader io.Reader, sheetName string, do func(colIdx int, row []string) error) error {
	if reader == nil {
		return errors.New("excelx: reader is nil")
	}
	tmp, err := os.CreateTemp("", "yunka-excelx-*.xlsx")
	if err != nil {
		return fmt.Errorf("excelx: create temporary workbook: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)

	if _, err := io.Copy(tmp, reader); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("excelx: spool workbook: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("excelx: close temporary workbook: %w", err)
	}
	return ExcelFromFile(name, sheetName, do)
}

func ExcelFromFile(fileName string, sheetName string, do func(colIdx int, row []string) error) error {
	if do == nil {
		return errors.New("excelx: row callback is nil")
	}
	archive, err := zip.OpenReader(fileName)
	if err != nil {
		return fmt.Errorf("excelx: open workbook: %w", err)
	}
	defer archive.Close()
	return readWorkbookRows(archive.File, sheetName, do)
}

type workbookSheet struct {
	Name  string
	RelID string
}

type workbookInfo struct {
	ActiveTab int
	Sheets    []workbookSheet
}

type workbookXML struct {
	BookViews struct {
		Views []struct {
			ActiveTab int `xml:"activeTab,attr"`
		} `xml:"workbookView"`
	} `xml:"bookViews"`
	Sheets []struct {
		Name  string `xml:"name,attr"`
		RelID string `xml:"id,attr"`
	} `xml:"sheets>sheet"`
}

type relsXML struct {
	Relationships []struct {
		ID     string `xml:"Id,attr"`
		Target string `xml:"Target,attr"`
		Type   string `xml:"Type,attr"`
	} `xml:"Relationship"`
}

func readWorkbookRows(files []*zip.File, requestedSheet string, do func(colIdx int, row []string) error) error {
	index := make(map[string]*zip.File, len(files))
	for _, file := range files {
		index[path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))] = file
	}

	info, err := loadWorkbookInfo(index)
	if err != nil {
		return err
	}
	targetSheet, err := chooseSheet(info, requestedSheet)
	if err != nil {
		return err
	}
	relationships, err := loadWorkbookRelationships(index)
	if err != nil {
		return err
	}
	target, ok := relationships[targetSheet.RelID]
	if !ok {
		return fmt.Errorf("excelx: relationship %q for sheet %q is missing", targetSheet.RelID, targetSheet.Name)
	}
	worksheetPath, err := resolveWorkbookTarget(target)
	if err != nil {
		return err
	}
	worksheet, ok := index[worksheetPath]
	if !ok {
		return fmt.Errorf("excelx: worksheet %q for sheet %q is missing", worksheetPath, targetSheet.Name)
	}
	shared, err := loadSharedStrings(index)
	if err != nil {
		return err
	}
	return streamWorksheet(worksheet, shared, do)
}

func loadWorkbookInfo(index map[string]*zip.File) (workbookInfo, error) {
	file, ok := index["xl/workbook.xml"]
	if !ok {
		return workbookInfo{}, errors.New("excelx: xl/workbook.xml is missing")
	}
	if err := checkPartSize(file, maxWorkbookPartSize); err != nil {
		return workbookInfo{}, err
	}
	reader, err := file.Open()
	if err != nil {
		return workbookInfo{}, fmt.Errorf("excelx: open workbook metadata: %w", err)
	}
	defer reader.Close()

	var document workbookXML
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return workbookInfo{}, fmt.Errorf("excelx: parse workbook metadata: %w", err)
	}
	info := workbookInfo{}
	if len(document.BookViews.Views) > 0 {
		info.ActiveTab = document.BookViews.Views[0].ActiveTab
	}
	for _, sheet := range document.Sheets {
		info.Sheets = append(info.Sheets, workbookSheet{Name: sheet.Name, RelID: sheet.RelID})
	}
	if len(info.Sheets) == 0 {
		return workbookInfo{}, errors.New("excelx: workbook has no worksheets")
	}
	if info.ActiveTab < 0 || info.ActiveTab >= len(info.Sheets) {
		info.ActiveTab = 0
	}
	return info, nil
}

func loadWorkbookRelationships(index map[string]*zip.File) (map[string]string, error) {
	file, ok := index["xl/_rels/workbook.xml.rels"]
	if !ok {
		return nil, errors.New("excelx: xl/_rels/workbook.xml.rels is missing")
	}
	if err := checkPartSize(file, maxWorkbookPartSize); err != nil {
		return nil, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("excelx: open workbook relationships: %w", err)
	}
	defer reader.Close()

	var document relsXML
	if err := xml.NewDecoder(reader).Decode(&document); err != nil {
		return nil, fmt.Errorf("excelx: parse workbook relationships: %w", err)
	}
	result := map[string]string{}
	for _, relationship := range document.Relationships {
		if relationship.ID != "" && strings.Contains(relationship.Type, "/worksheet") {
			result[relationship.ID] = relationship.Target
		}
	}
	return result, nil
}

func chooseSheet(info workbookInfo, requested string) (workbookSheet, error) {
	if requested == "" {
		return info.Sheets[info.ActiveTab], nil
	}
	for _, sheet := range info.Sheets {
		if sheet.Name == requested {
			return sheet, nil
		}
	}
	return workbookSheet{}, fmt.Errorf("excelx: worksheet %q does not exist", requested)
}

func resolveWorkbookTarget(target string) (string, error) {
	target = strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	if target == "" {
		return "", errors.New("excelx: worksheet relationship target is empty")
	}
	var clean string
	if strings.HasPrefix(target, "/") {
		clean = path.Clean(strings.TrimPrefix(target, "/"))
	} else {
		clean = path.Clean(path.Join("xl", target))
	}
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || !strings.HasPrefix(clean, "xl/") {
		return "", fmt.Errorf("excelx: worksheet relationship target %q escapes xl/", target)
	}
	return clean, nil
}

func checkPartSize(file *zip.File, max uint64) error {
	if file.UncompressedSize64 > max {
		return fmt.Errorf("excelx: workbook part %q is too large: %d > %d", file.Name, file.UncompressedSize64, max)
	}
	return nil
}

func loadSharedStrings(index map[string]*zip.File) ([]string, error) {
	file, ok := index["xl/sharedStrings.xml"]
	if !ok {
		return []string{}, nil
	}
	if err := checkPartSize(file, maxSharedStringsSize); err != nil {
		return nil, err
	}
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("excelx: open shared strings: %w", err)
	}
	defer reader.Close()

	decoder := xml.NewDecoder(reader)
	result := []string{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("excelx: parse shared strings: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		value, err := collectRichText(decoder, start.Name)
		if err != nil {
			return nil, fmt.Errorf("excelx: parse shared string: %w", err)
		}
		result = append(result, value)
	}
	return result, nil
}

func collectRichText(decoder *xml.Decoder, end xml.Name) (string, error) {
	var builder strings.Builder
	phoneticDepth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "rPh" {
				phoneticDepth++
				continue
			}
			if value.Name.Local == "t" && phoneticDepth == 0 {
				var text string
				if err := decoder.DecodeElement(&text, &value); err != nil {
					return "", err
				}
				builder.WriteString(text)
			}
		case xml.EndElement:
			if value.Name.Local == "rPh" && phoneticDepth > 0 {
				phoneticDepth--
				continue
			}
			if value.Name == end {
				return builder.String(), nil
			}
		}
	}
}

type worksheetCell struct {
	Ref   string
	Type  string
	Value string
}

func streamWorksheet(file *zip.File, shared []string, do func(int, []string) error) error {
	if err := checkPartSize(file, maxWorksheetPartSize); err != nil {
		return err
	}
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("excelx: open worksheet %q: %w", file.Name, err)
	}
	defer reader.Close()

	decoder := xml.NewDecoder(reader)
	nextRow := 1
	pendingEmpty := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("excelx: parse worksheet %q: %w", file.Name, err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}

		rowNumber := nextRow
		for _, attribute := range start.Attr {
			if attribute.Name.Local != "r" || attribute.Value == "" {
				continue
			}
			parsed, parseErr := strconv.Atoi(attribute.Value)
			if parseErr != nil || parsed < 1 || parsed > maxExcelRows {
				return fmt.Errorf("excelx: invalid row number %q", attribute.Value)
			}
			rowNumber = parsed
			break
		}
		if rowNumber < nextRow {
			return fmt.Errorf("excelx: worksheet row order regressed from %d to %d", nextRow, rowNumber)
		}
		pendingEmpty += rowNumber - nextRow

		row, err := parseRow(decoder, start.Name, shared)
		if err != nil {
			return err
		}
		if len(row) == 0 {
			pendingEmpty++
			nextRow = rowNumber + 1
			continue
		}
		for pendingEmpty > 0 {
			if err := do(nextRow-pendingEmpty, []string{}); err != nil {
				return err
			}
			pendingEmpty--
		}
		if err := do(rowNumber-1, row); err != nil {
			return err
		}
		nextRow = rowNumber + 1
	}
	return nil
}

func parseRow(decoder *xml.Decoder, end xml.Name, shared []string) ([]string, error) {
	row := []string{}
	nextColumn := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local != "c" {
				continue
			}
			cell, err := parseCell(decoder, value, shared)
			if err != nil {
				return nil, err
			}
			column := nextColumn
			if cell.Ref != "" {
				parsed, err := columnIndex(cell.Ref)
				if err != nil {
					return nil, err
				}
				column = parsed
			}
			if column < nextColumn {
				return nil, fmt.Errorf("excelx: cell %q is out of column order", cell.Ref)
			}
			for len(row) <= column {
				row = append(row, "")
			}
			row[column] = cell.Value
			nextColumn = column + 1
		case xml.EndElement:
			if value.Name == end {
				for len(row) > 0 && row[len(row)-1] == "" {
					row = row[:len(row)-1]
				}
				return row, nil
			}
		}
	}
}

func parseCell(decoder *xml.Decoder, start xml.StartElement, shared []string) (worksheetCell, error) {
	cell := worksheetCell{}
	for _, attribute := range start.Attr {
		switch attribute.Name.Local {
		case "r":
			cell.Ref = attribute.Value
		case "t":
			cell.Type = attribute.Value
		}
	}

	raw := ""
	inline := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return cell, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "v":
				if err := decoder.DecodeElement(&raw, &value); err != nil {
					return cell, err
				}
			case "is":
				text, err := collectRichText(decoder, value.Name)
				if err != nil {
					return cell, err
				}
				inline = text
			default:
				if err := decoder.Skip(); err != nil {
					return cell, err
				}
			}
		case xml.EndElement:
			if value.Name == start.Name {
				decoded, err := decodeCellValue(cell.Type, raw, inline, shared)
				cell.Value = decoded
				return cell, err
			}
		}
	}
}

func decodeCellValue(kind, raw, inline string, shared []string) (string, error) {
	switch kind {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || index < 0 || index >= len(shared) {
			return "", fmt.Errorf("excelx: invalid shared string index %q", raw)
		}
		return shared[index], nil
	case "inlineStr":
		return inline, nil
	case "b":
		switch strings.TrimSpace(raw) {
		case "1":
			return "TRUE", nil
		case "0":
			return "FALSE", nil
		}
	}
	return raw, nil
}

func columnIndex(reference string) (int, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return 0, errors.New("excelx: empty cell reference")
	}
	number := 0
	letters := 0
	for _, character := range reference {
		if !unicode.IsLetter(character) {
			break
		}
		if character >= 'a' && character <= 'z' {
			character -= 'a' - 'A'
		}
		if character < 'A' || character > 'Z' {
			return 0, fmt.Errorf("excelx: invalid cell reference %q", reference)
		}
		number = number*26 + int(character-'A'+1)
		letters++
	}
	if letters == 0 || number < 1 || number > maxExcelColumns {
		return 0, fmt.Errorf("excelx: invalid cell reference %q", reference)
	}
	return number - 1, nil
}
'''

tests = r'''package excelx

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
'''

(ROOT / "pkg/excelx/reader.go").write_text(reader)
(ROOT / "pkg/excelx/reader_test.go").write_text(tests)
print("framework-local excelx safe reader prepared")
