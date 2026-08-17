package excelx

import (
	"github.com/xuri/excelize/v2"
	"io"
)

func ExcelFromIOReader(reader io.Reader, sheetName string, do func(colIdx int, row []string) error) error {
	f, err := excelize.OpenReader(reader)
	if err != nil {
		return err
	}
	defer f.Close()
	return doExcelReader(f, sheetName, do)
}

func ExcelFromFile(f string, sheetName string, do func(colIdx int, row []string) error) error {
	file, err := excelize.OpenFile(f)
	if err != nil {
		return err
	}
	defer file.Close()
	return doExcelReader(file, sheetName, do)
}

func doExcelReader(f *excelize.File, sheetName string, do func(colIdx int, row []string) error) error {
	if sheetName == "" {
		sheetName = f.GetSheetName(f.GetActiveSheetIndex())
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return err
	}
	if rows == nil || len(rows) == 0 {
		return nil
	}
	for i := 0; i < len(rows); i++ {
		if err = do(i, rows[i]); err != nil {
			return err
		}
	}
	return nil
}
