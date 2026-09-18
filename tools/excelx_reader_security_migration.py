#!/usr/bin/env python3
from pathlib import Path

root = Path.cwd()

reader = r'''package excelx

import (
    "errors"
    "fmt"
    "io"

    "codeberg.org/tealeg/xlsx/v4"
)

const excelMaxColumns = 16384

func workbookOptions() []xlsx.FileOption {
    return []xlsx.FileOption{
        xlsx.RowLimit(xlsx.Excel2006MaxRowCount),
        xlsx.ColLimit(excelMaxColumns),
    }
}

func ExcelFromIOReader(reader io.Reader, sheetName string, do func(colIdx int, row []string) error) error {
    if reader == nil {
        return errors.New("excelx: reader is required")
    }
    contents, err := io.ReadAll(reader)
    if err != nil {
        return err
    }
    file, err := xlsx.OpenBinary(contents, workbookOptions()...)
    if err != nil {
        return err
    }
    return doExcelReader(file, sheetName, do)
}

func ExcelFromFile(path string, sheetName string, do func(colIdx int, row []string) error) error {
    file, err := xlsx.OpenFile(path, workbookOptions()...)
    if err != nil {
        return err
    }
    return doExcelReader(file, sheetName, do)
}

func doExcelReader(file *xlsx.File, sheetName string, do func(colIdx int, row []string) error) error {
    if do == nil {
        return errors.New("excelx: row callback is required")
    }
    sheet, err := selectSheet(file, sheetName)
    if err != nil {
        return err
    }

    nextRow := 0
    return sheet.ForEachRow(func(row *xlsx.Row) error {
        rowIndex := row.GetCoordinate()
        if rowIndex < nextRow {
            return fmt.Errorf("excelx: worksheet row order moved backwards: row=%d next=%d", rowIndex+1, nextRow+1)
        }
        values, err := formattedRow(row)
        if err != nil {
            return fmt.Errorf("excelx: format row %d: %w", rowIndex+1, err)
        }
        // Excelize GetRows omits trailing empty rows, but materializes empty
        // gaps once a later non-empty row exists. Preserve that public behavior.
        if len(values) == 0 {
            return nil
        }
        for nextRow < rowIndex {
            if err := do(nextRow, nil); err != nil {
                return err
            }
            nextRow++
        }
        if err := do(rowIndex, values); err != nil {
            return err
        }
        nextRow = rowIndex + 1
        return nil
    })
}

func selectSheet(file *xlsx.File, sheetName string) (*xlsx.Sheet, error) {
    if file == nil {
        return nil, errors.New("excelx: workbook is required")
    }
    if sheetName != "" {
        sheet, ok := file.Sheet[sheetName]
        if !ok || sheet == nil {
            return nil, fmt.Errorf("excelx: sheet %q does not exist", sheetName)
        }
        return sheet, nil
    }
    for _, sheet := range file.Sheets {
        if sheet != nil && sheet.Selected {
            return sheet, nil
        }
    }
    for _, sheet := range file.Sheets {
        if sheet != nil {
            return sheet, nil
        }
    }
    return nil, errors.New("excelx: workbook has no worksheets")
}

func formattedRow(row *xlsx.Row) ([]string, error) {
    values := []string(nil)
    err := row.ForEachCell(func(cell *xlsx.Cell) error {
        column, _ := cell.GetCoordinates()
        if column < 0 || column >= excelMaxColumns {
            return fmt.Errorf("cell column %d is outside Excel limits", column+1)
        }
        value, err := cell.FormattedValue()
        if err != nil {
            return err
        }
        if value == "" && !cell.HasFormula() {
            return nil
        }
        if missing := column + 1 - len(values); missing > 0 {
            values = append(values, make([]string, missing)...)
        }
        values[column] = value
        return nil
    })
    return values, err
}
'''

tests = r'''package excelx

import (
    "bytes"
    "errors"
    "os"
    "path/filepath"
    "reflect"
    "strings"
    "testing"

    "codeberg.org/tealeg/xlsx/v4"
)

func TestExcelReaderPreservesSelectedSheetAndSparseRows(t *testing.T) {
    contents := testWorkbook(t)

    var selected [][]string
    if err := ExcelFromIOReader(bytes.NewReader(contents), "", func(_ int, row []string) error {
        selected = append(selected, append([]string(nil), row...))
        return nil
    }); err != nil {
        t.Fatal(err)
    }
    if want := [][]string{{"selected"}}; !reflect.DeepEqual(selected, want) {
        t.Fatalf("selected sheet rows=%#v want=%#v", selected, want)
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
    first.Selected = false
    selected.Selected = true
    selected.AddRow().AddCell().SetString("selected")

    var buffer bytes.Buffer
    if err := file.Write(&buffer); err != nil {
        t.Fatal(err)
    }
    return buffer.Bytes()
}
'''

(root / "pkg/excelx/reader.go").write_text(reader)
(root / "pkg/excelx/reader_test.go").write_text(tests)
print("excelx reader migration prepared")
