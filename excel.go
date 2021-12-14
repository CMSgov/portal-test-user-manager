package main

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

const (
	colUser = iota
	colPortal
	colPrevious
	colTimestamp
	numCols
)

const automatedSheet string = "PasswordManager"

func getHeaderToXCoord(headerRow []string) map[string]int {
	headerToXCoord := make(map[string]int, len(headerRow))
	for i, cell := range headerRow {
		headerToXCoord[cell] = i
	}
	return headerToXCoord
}

func getMACFinUsers(f *excelize.File, input *Input) (map[string]string, error) {
	rows, err := f.GetRows(input.SheetName)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in %s: %s", input.SheetName, input.Filename, err)
	}

	users := make(map[string]string)

	headerToXCoord := getHeaderToXCoord(rows[0])
	usernameXCoord := headerToXCoord[input.UsernameHeader]
	passwordXCoord := headerToXCoord[input.PasswordHeader]

	for _, row := range rows[rowOffset:] {
		// check if row is empty
		if row == nil {
			continue
		}
		users[row[usernameXCoord]] = row[passwordXCoord]
	}

	return users, nil
}

func getPasswordManagerUsers(f *excelize.File, input *Input) (map[string]string, error) {
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in %s: %s", automatedSheet, input.Filename, err)
	}

	passwordManagerUsersToPassword := make(map[string]string)
	for _, row := range rows[rowOffset:] {
		passwordManagerUsersToPassword[row[colUser]] = row[colPortal]
	}

	return passwordManagerUsersToPassword, nil
}

// Sync PasswordManager usernames with MACFin users
func syncPasswordManagerUsersToMACFinUsers(f *excelize.File, input *Input) error {
	macFinUsersToPasswords, err := getMACFinUsers(f, input)
	if err != nil {
		return err
	}

	passwordManagerUsersToPassword, err := getPasswordManagerUsers(f, input)
	if err != nil {
		return err
	}

	numRows := len(passwordManagerUsersToPassword)

	// add new MACFin users to automatedSheet
	for user, password := range macFinUsersToPasswords {
		if _, ok := passwordManagerUsersToPassword[user]; !ok {
			values := [numCols]string{user, password, password, "Rotate Now"}
			f.InsertRow(automatedSheet, numRows+1) // insert before numRows+1
			numRows++
			// write row
			for j := 0; j < numCols; j++ {
				err := writeCell(f, input.Filename, automatedSheet, j+sheetOffset, numRows, values[j])
				if err != nil {
					return fmt.Errorf("failed adding new MACFin user %s to %s sheet in file %s: %s", user, automatedSheet, input.Filename, err)
				}
			}
			continue
		}
	}

	// remove deleted MACFin users from PasswordManager sheet
	rowIndx := 1
	for pwUser := range passwordManagerUsersToPassword {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MACFin users; remove pwUser from PasswordManager
			err := f.RemoveRow(automatedSheet, rowIndx+rowOffset)
			if err != nil {
				return fmt.Errorf("failed removing user %s from %s sheet in file %s: %s", pwUser, automatedSheet, input.Filename, err)
			}
			continue
		}
		rowIndx++
	}

	err = f.SaveAs(input.Filename)
	if err != nil {
		return fmt.Errorf("failed saving %s after synchronizing automated sheet users to MACFin users: %s", input.Filename, err)
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

func validateFileSize(f *excelize.File, input *Input) (errors []string) {
	v := new(Validator)

	rows, err := f.GetRows(input.SheetName)
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
func updateMACFinUsers(f *excelize.File, input *Input) error {
	passwordManagerUsersToPassword, err := getPasswordManagerUsers(f, input)
	if err != nil {
		return err
	}

	rows, err := f.GetRows(input.SheetName)
	if err != nil {
		return err
	}

	numPasswordsUpdated := 0
	headerNameToXCoord := getHeaderToXCoord(rows[0])
	userX := headerNameToXCoord[input.UsernameHeader]
	passwordX := headerNameToXCoord[input.PasswordHeader]

	for i, row := range rows[rowOffset:] {
		if row == nil {
			continue
		}
		user := row[userX]
		macPassword := row[passwordX]
		if password, ok := passwordManagerUsersToPassword[user]; ok {
			if password != macPassword {
				err := writeCell(f, input.Filename, input.SheetName, passwordX+sheetOffset, i+rowOffset+sheetOffset, password)
				if err != nil {
					return fmt.Errorf("error setting new password for user %s in sheet %s on row %d: %s", user, input.SheetName, i+rowOffset+sheetOffset, err)
				}
				numPasswordsUpdated++
			}
		} else {
			return fmt.Errorf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new passwords", user, input.SheetName)
		}
	}

	return nil
}
