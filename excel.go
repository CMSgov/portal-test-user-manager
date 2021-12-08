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
		config.errorLog.Print("failed reading rows to synchronize users")
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
				err := writeCell(f, config, automatedSheet, j+sheetOffset, numRows, values[j])
				if err != nil {
					config.errorLog.Printf("failed adding new macFin user %s to automated sheet", user)
					return err
				}
			}
			continue
		}
	}

	// remove deleted MACFIN users from PasswordManager sheet
	rowIndx := 1
	for pwUser := range passwordManagerUsers {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MCFIN users; remove pwUser from PasswordManager
			err := f.RemoveRow(automatedSheet, rowIndx+rowOffset)
			if err != nil {
				config.errorLog.Printf("failed removing user %s from automated sheet: ", pwUser)
				return err
			}
			continue
		}
		rowIndx++
	}

	err = f.SaveAs(config.Filename)
	if err != nil {
		config.errorLog.Print("failed saving file after synchronizing automated sheet users to macFin users")
		return err
	}

	return nil
}

// check all errors; save file after every cell update
func writeCell(f *excelize.File, config *Portal, sheet string, xCoord, yCoord int, value string) error {
	cellName, err := excelize.CoordinatesToCellName(xCoord, yCoord)
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, cellName, value)
	if err != nil {
		return err
	}
	err = f.SaveAs(config.Filename)
	if err != nil {
		return err
	}

	return nil
}

// Copy cell in automatedSheet
func copyCell(f *excelize.File, config *Portal, srcX, srcY, destX, destY int) error {
	srcCell, err := excelize.CoordinatesToCellName(srcX, srcY)
	if err != nil {
		return err
	}
	destCell, err := excelize.CoordinatesToCellName(destX, destY)
	if err != nil {
		return err
	}
	srcValue, err := f.GetCellValue(automatedSheet, srcCell)
	if err != nil {
		return err
	}
	err = f.SetCellValue(automatedSheet, destCell, srcValue)
	if err != nil {
		return err
	}
	err = f.SaveAs(config.Filename)
	if err != nil {
		return err
	}

	return nil
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
