package main

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

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
	rowOffset := input.rowOffset

	for _, row := range rows[rowOffset:] {
		// check if row is empty
		if row == nil {
			continue
		}
		users[row[usernameXCoord]] = row[passwordXCoord]
	}

	return users, nil
}

func getManagedUsers(f *excelize.File, input *Input) (usersToPassword map[string]string, usersToRow map[string]int, err error) {
	automatedSheet := input.automatedSheetName
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return nil, nil, fmt.Errorf("failed getting rows from %s in %s: %s", automatedSheet, input.Filename, err)
	}

	rowOffset := input.rowOffset
	colUser := input.automatedSheetColNameToIndex["colUser"]
	colPortal := input.automatedSheetColNameToIndex["colPortal"]

	usersToPassword = make(map[string]string)
	usersToRow = make(map[string]int)
	for i, row := range rows[rowOffset:] {
		usersToPassword[row[colUser]] = row[colPortal]
		usersToRow[row[colUser]] = i
	}

	return usersToPassword, usersToRow, nil
}

// Sync PasswordManager usernames with MACFin users
func syncPasswordManagerUsersToMACFinUsers(f *excelize.File, input *Input) error {
	macFinUsersToPasswords, err := getMACFinUsers(f, input)
	if err != nil {
		return err
	}

	_, usersToRow, err := getManagedUsers(f, input)
	if err != nil {
		return err
	}

	rowOffset := input.rowOffset
	sheetOffset := input.sheetOffset
	numRows := len(usersToRow) + rowOffset
	automatedSheet := input.automatedSheetName
	numCols := len(input.automatedSheetColNameToIndex)
	colDelete := input.automatedSheetColNameToIndex["colDelete"]

	// add new MACFin users to automatedSheet
	for macFinUser, password := range macFinUsersToPasswords {
		if _, ok := usersToRow[macFinUser]; !ok {
			values := []string{macFinUser, password, password, "Rotate Now", ""}
			// write new row
			for col := 0; col < numCols; col++ {
				err := writeCell(f, automatedSheet, col+sheetOffset, numRows+1, values[col])
				if err != nil {
					return fmt.Errorf("failed adding new MACFin user %s to %s sheet in file %s: %s", macFinUser, automatedSheet, input.Filename, err)
				}
			}
			numRows++
			continue
		}
	}

	// mark users for deletion from automatedSheet
	for pwUser, row := range usersToRow {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MACFin users; mark pwUser for deletion from automatedSheet
			err := writeCell(f, automatedSheet, colDelete+sheetOffset, row+rowOffset+sheetOffset, "delete")
			if err != nil {
				return fmt.Errorf("failed to mark user %s from sheet %s for deletion: %s", pwUser, automatedSheet, err)
			}
			continue
		}
	}

	// delete users from automatedSheet that are marked for deletion
	// iterate from bottom row to top
	for i := numRows; i >= rowOffset+sheetOffset; i-- {
		value, err := getCellValue(f, automatedSheet, colDelete+sheetOffset, i)
		if err != nil {
			return fmt.Errorf("failed to read cell value from col %d in sheet %s: %s", colDelete+rowOffset+sheetOffset, automatedSheet, err)
		}
		if value == "delete" {
			err := f.RemoveRow(automatedSheet, i)
			if err != nil {
				return fmt.Errorf("failed removing row %d from %s sheet in file %s: %s", i, automatedSheet, input.Filename, err)
			}
		}
	}

	err = f.Save()
	if err != nil {
		return fmt.Errorf("failed saving %s after synchronizing automated sheet users to MACFin users: %s", input.Filename, err)
	}

	return nil
}

func getCellValue(f *excelize.File, sheet string, xCoord, yCoord int) (string, error) {
	cellName, err := excelize.CoordinatesToCellName(xCoord, yCoord)
	if err != nil {
		return "", err
	}
	value, err := f.GetCellValue(sheet, cellName)
	if err != nil {
		return "", err
	}
	return value, nil
}

func writeCell(f *excelize.File, sheet string, xCoord, yCoord int, value string) error {
	cellName, err := excelize.CoordinatesToCellName(xCoord, yCoord)
	if err != nil {
		return err
	}
	err = f.SetCellValue(sheet, cellName, value)
	if err != nil {
		return err
	}
	err = f.Save()
	if err != nil {
		return err
	}

	return nil
}

// Copy cell in automatedSheet
func copyCell(f *excelize.File, automatedSheetName string, srcX, srcY, destX, destY int) error {
	srcCell, err := excelize.CoordinatesToCellName(srcX, srcY)
	if err != nil {
		return err
	}
	destCell, err := excelize.CoordinatesToCellName(destX, destY)
	if err != nil {
		return err
	}
	srcValue, err := f.GetCellValue(automatedSheetName, srcCell)
	if err != nil {
		return err
	}
	err = f.SetCellValue(automatedSheetName, destCell, srcValue)
	if err != nil {
		return err
	}
	err = f.Save()
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
	passwordManagerUsersToPassword, _, err := getManagedUsers(f, input)
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
	rowOffset := input.rowOffset
	sheetOffset := input.sheetOffset

	for i, row := range rows[rowOffset:] {
		if row == nil {
			continue
		}
		user := row[userX]
		macPassword := row[passwordX]
		if password, ok := passwordManagerUsersToPassword[user]; ok {
			if password != macPassword {
				err := writeCell(f, input.SheetName, passwordX+sheetOffset, i+rowOffset+sheetOffset, password)
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
