package main

import (
	"github.com/xuri/excelize/v2"
)

const (
	user = iota
	portal
	previous
	timestamp
	numCols        int    = 4
	automatedSheet string = "PasswordManager"
)

func getMCFinUsers(f *excelize.File, config *Portal) (map[string]string, error) {
	rows, err := f.GetRows(config.SheetName)
	if err != nil {
		config.errorLog.Printf("failed getting rows from %s in %s", config.SheetName, config.Filename)
		return nil, err
	}

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

func getPasswordManagerUsers(f *excelize.File, config *Portal) (map[string]string, error) {
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		config.errorLog.Printf("failed getting rows from %s in %s", automatedSheet, config.Filename)
		return nil, err
	}

	passwordManagerUsersToPassword := make(map[string]string)
	for _, row := range rows[rowOffset:] {
		passwordManagerUsersToPassword[row[user]] = row[portal]
	}

	return passwordManagerUsersToPassword, nil
}

// Sync PasswordManager usernames with MACFIN users
func syncPasswordManagerUsersToMACFINUsers(f *excelize.File, config *Portal) error {
	macFinUsersToPasswords, err := getMCFinUsers(f, config)
	if err != nil {
		return err
	}

	passwordManagerUsersToPassword, err := getPasswordManagerUsers(f, config)
	if err != nil {
		config.errorLog.Printf("failed reading users from %s", automatedSheet)
		return err
	}

	numRows := len(passwordManagerUsersToPassword)

	// add new MACFIN users to automatedSheet
	for user, password := range macFinUsersToPasswords {
		if _, ok := passwordManagerUsersToPassword[user]; !ok {
			values := [numCols]string{user, password, password, "Rotate Now"}
			f.InsertRow(automatedSheet, numRows+1) // insert before numRows+1
			numRows++
			// write row
			for j := 0; j < numCols; j++ {
				err := writeCell(f, config.Filename, automatedSheet, j+sheetOffset, numRows, values[j])
				if err != nil {
					config.errorLog.Printf("failed adding new macFin user %s to %s sheet in file %s",
						user, automatedSheet, config.Filename)
					return err
				}
			}
			continue
		}
	}

	// remove deleted MACFIN users from PasswordManager sheet
	rowIndx := 1
	for pwUser := range passwordManagerUsersToPassword {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MCFIN users; remove pwUser from PasswordManager
			err := f.RemoveRow(automatedSheet, rowIndx+rowOffset)
			if err != nil {
				config.errorLog.Printf("failed removing user %s from %s sheet in file %s", pwUser, automatedSheet, config.Filename)
				return err
			}
			continue
		}
		rowIndx++
	}

	err = f.SaveAs(config.Filename)
	if err != nil {
		config.errorLog.Printf("failed saving %s after synchronizing automated sheet users to macFin users", config.Filename)
		return err
	}

	return nil
}

func writeCell(f *excelize.File, filename, sheet string, xCoord, yCoord int, value string) error {
	cellName, err := excelize.CoordinatesToCellName(xCoord, yCoord)
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, cellName, value)
	if err != nil {
		return err
	}
	err = f.SaveAs(filename)
	if err != nil {
		return err
	}

	return nil
}

// Copy cell in automatedSheet
func copyCell(f *excelize.File, filename string, srcX, srcY, destX, destY int) error {
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
	err = f.SaveAs(filename)
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

// Write new password to password column in the testing sheet
func updateMacFinUsers(f *excelize.File, config *Portal) error {
	passwordManagerUsersToPassword, err := getPasswordManagerUsers(f, config)
	if err != nil {
		return err
	}

	rows, err := f.GetRows(config.SheetName)
	if err != nil {
		return err
	}

	numPasswordsUpdated := 0
	headerNameToXCoord := getHeaderToXCoord(rows[0])
	userX := headerNameToXCoord[config.UsernameHeader]
	passwordX := headerNameToXCoord[config.PasswordHeader]

	for i, row := range rows[rowOffset:] {
		if row == nil {
			continue
		}
		user := row[userX]
		macPassword := row[passwordX]
		if password, ok := passwordManagerUsersToPassword[user]; ok {
			if password != macPassword {
				err := writeCell(f, config.Filename, config.SheetName, passwordX+sheetOffset, i+rowOffset+sheetOffset, password)
				if err != nil {
					config.errorLog.Printf("error setting new password for user %s in sheet %s on row %d",
						user, config.SheetName, i+rowOffset+sheetOffset)
					return err
				}
				numPasswordsUpdated++
			}
		} else {
			config.errorLog.Printf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new passwords",
				user, config.SheetName)
			return err
		}
	}
	config.infoLog.Printf("successfully updated %d passwords for users in sheet %s in file %s",
		numPasswordsUpdated, config.SheetName, config.Filename)
	return nil
}
