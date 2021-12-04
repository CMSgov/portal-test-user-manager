package main

import (
	"github.com/xuri/excelize/v2"
)

const (
	user = iota
	local
	portal
	previous
	timestamp
	automatedSheet = "PasswordManager"
)

func getMCFinUsers(f *excelize.File, config *Portal) (map[string]string, error) {
	sheetName := config.SheetName
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	rowOffset := 1 // row 0 is the header row

	users := make(map[string]string)

	headerToXCoord := getHeaderToXCoord(rows[0])
	usernameXCoord := headerToXCoord[config.UsernameHeader]
	passwordXCoord := headerToXCoord[config.PasswordHeader]

	for _, row := range rows[rowOffset:] {
		// check if row is empty
		if row == nil {
			continue
		}
		users[row[usernameXCoord]] = row[passwordXCoord]
	}

	return users, nil
}

// Sync PasswordManager usernames with MACFIN users
func syncPasswordManagerUsersToMACFINUsers(f *excelize.File, config *Portal) error {
	macFinUsersToPasswords, err := getMCFinUsers(f, config)
	if err != nil {
		return err
	}

	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return err
	}

	numRows := len(rows)
	rowOffset := 1   // row 0 is the header row
	sheetOffset := 1 // col numbering starts from 1

	passwordManagerUsers := make(map[string]string)
	for _, row := range rows[rowOffset:] {
		passwordManagerUsers[row[user]] = row[portal]
	}

	// add new MACFIN users to automatedSheet
	for user, password := range macFinUsersToPasswords {
		if _, ok := passwordManagerUsers[user]; !ok {
			values := []string{user, password, password, password, "Rotate Now"}
			f.InsertRow(automatedSheet, numRows+1) // insert before numRows+1
			numRows++
			// write row
			for j := 0; j < 5; j++ {
				writeCell(f, automatedSheet, j+sheetOffset, numRows, values[j])
			}
			continue
		}
	}

	// remove deleted MACFIN users from PasswordManager sheet
	rowIndx := 1
	for pwUser := range passwordManagerUsers {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MCFIN users; remove pwUser from PasswordManager
			f.RemoveRow(automatedSheet, rowIndx+rowOffset)
			continue
		}
		rowIndx++
	}

	err = f.SaveAs(config.Filename)
	if err != nil {
		return err
	}

	return nil
}

func writeCell(f *excelize.File, sheet string, xCoord, yCoord int, value string) {
	cellName, _ := excelize.CoordinatesToCellName(xCoord, yCoord)
	f.SetCellValue(sheet, cellName, value)
}

func copyCell(f *excelize.File, automatedSheet string, srcX, srcY, destX, destY int) {
	srcCell, _ := excelize.CoordinatesToCellName(srcX, srcY)
	destCell, _ := excelize.CoordinatesToCellName(destX, destY)
	srcValue, _ := f.GetCellValue(automatedSheet, srcCell)
	f.SetCellValue(automatedSheet, destCell, srcValue)
}

func validateFileSize(f *excelize.File, config *Input) (errors []string) {
	v := new(Validator)

	rows, err := f.GetRows(config.SheetName)
	if err != nil {
		v.Errorf("Error getting rows: %s", err)

	}
	if len(rows) > excelize.TotalRows {
		v.Errorf("Number of rows %d exceeds max number %d", len(rows), excelize.TotalRows)
	}

	if len(rows[0]) > excelize.TotalColumns {
		v.Errorf("Number of columns %d exceeds max number %d", len(rows[0]), excelize.TotalColumns)
	}

	return v.Errors
}
