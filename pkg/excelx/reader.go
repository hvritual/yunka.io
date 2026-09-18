package excelx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
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
	activeSheet, err := workbookActiveSheetNameFromBytes(contents)
	if err != nil {
		return err
	}
	file, err := xlsx.OpenBinary(contents, workbookOptions()...)
	if err != nil {
		return err
	}
	return doExcelReader(file, sheetName, activeSheet, do)
}

func ExcelFromFile(path string, sheetName string, do func(colIdx int, row []string) error) error {
	activeSheet, err := workbookActiveSheetNameFromFile(path)
	if err != nil {
		return err
	}
	file, err := xlsx.OpenFile(path, workbookOptions()...)
	if err != nil {
		return err
	}
	return doExcelReader(file, sheetName, activeSheet, do)
}

func doExcelReader(file *xlsx.File, sheetName, activeSheet string, do func(colIdx int, row []string) error) error {
	if do == nil {
		return errors.New("excelx: row callback is required")
	}
	sheet, err := selectSheet(file, sheetName, activeSheet)
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

func selectSheet(file *xlsx.File, sheetName, activeSheet string) (*xlsx.Sheet, error) {
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
	if activeSheet != "" {
		if sheet, ok := file.Sheet[activeSheet]; ok && sheet != nil {
			return sheet, nil
		}
		return nil, fmt.Errorf("excelx: active sheet %q is not a worksheet", activeSheet)
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

type workbookMetadata struct {
	Views  []workbookView  `xml:"bookViews>workbookView"`
	Sheets []workbookSheet `xml:"sheets>sheet"`
}

type workbookView struct {
	ActiveTab *int `xml:"activeTab,attr"`
}

type workbookSheet struct {
	Name string `xml:"name,attr"`
}

func workbookActiveSheetNameFromBytes(contents []byte) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	if err != nil {
		return "", err
	}
	return workbookActiveSheetName(archive.File)
}

func workbookActiveSheetNameFromFile(path string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	return workbookActiveSheetName(archive.File)
}

func workbookActiveSheetName(files []*zip.File) (string, error) {
	for _, entry := range files {
		if entry.Name != "xl/workbook.xml" {
			continue
		}
		reader, err := entry.Open()
		if err != nil {
			return "", err
		}
		var metadata workbookMetadata
		decodeErr := xml.NewDecoder(reader).Decode(&metadata)
		closeErr := reader.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("excelx: decode workbook metadata: %w", decodeErr)
		}
		if closeErr != nil {
			return "", closeErr
		}
		if len(metadata.Sheets) == 0 {
			return "", errors.New("excelx: workbook has no sheets")
		}
		activeIndex := 0
		if len(metadata.Views) > 0 && metadata.Views[0].ActiveTab != nil {
			activeIndex = *metadata.Views[0].ActiveTab
		}
		if activeIndex < 0 || activeIndex >= len(metadata.Sheets) {
			return "", fmt.Errorf("excelx: workbook active sheet index %d is outside sheet list", activeIndex)
		}
		return metadata.Sheets[activeIndex].Name, nil
	}
	return "", errors.New("excelx: workbook metadata xl/workbook.xml is missing")
}
