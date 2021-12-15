package main

import (
	"fmt"
	"sort"

	"github.com/xuri/excelize/v2"
)

type PasswordRow struct {
	Password string
	Row      int
}

func toSheetCoord(coord int) int {
	return coord + 1
}

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

func getManagedUsers(f *excelize.File, input *Input) (map[string]PasswordRow, error) {
	automatedSheet := input.automatedSheetName
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in %s: %s", automatedSheet, input.Filename, err)
	}

	rowOffset := input.rowOffset
	colUser := input.automatedSheetColNameToIndex["colUser"]
	colPortal := input.automatedSheetColNameToIndex["colPortal"]
	userToPasswordRow := make(map[string]PasswordRow)

	for i, row := range rows[rowOffset:] {
		userToPasswordRow[row[colUser]] = PasswordRow{
			Password: row[colPortal],
			Row:      i,
		}
	}

	return userToPasswordRow, nil
}

// Sync PasswordManager usernames with MACFin users
func syncPasswordManagerUsersToMACFinUsers(f *excelize.File, input *Input) error {
	macFinUsersToPasswords, err := getMACFinUsers(f, input)
	if err != nil {
		return err
	}

	userToPasswordRow, err := getManagedUsers(f, input)
	if err != nil {
		return err
	}

	rowOffset := input.rowOffset
	numRows := len(userToPasswordRow) + rowOffset
	automatedSheet := input.automatedSheetName
	numCols := len(input.automatedSheetColNameToIndex)

	// add new MACFin users to automatedSheet
	for macFinUser, password := range macFinUsersToPasswords {
		if _, ok := userToPasswordRow[macFinUser]; !ok {
			values := []string{macFinUser, password, password, "Rotate Now"}
			// write new row
			for col := 0; col < numCols; col++ {
				err := writeCell(f, automatedSheet, col, numRows, values[col])
				if err != nil {
					return fmt.Errorf("failed adding new MACFin user %s to %s sheet in file %s: %s", macFinUser, automatedSheet, input.Filename, err)
				}
			}
			numRows++
			continue
		}
	}

	// get rows for deletion from automatedSheet
	rowsToDelete := []int{}
	for pwUser, pwRow := range userToPasswordRow {
		if _, ok := macFinUsersToPasswords[pwUser]; !ok {
			// pwUser is not in MACFin users; mark row for deletion from automatedSheet
			rowsToDelete = append(rowsToDelete, pwRow.Row)
			continue
		}
	}

	// delete rows from automatedSheet that are marked for deletion
	// iterate in descending order
	sort.Ints(rowsToDelete)
	for idx := len(rowsToDelete) - 1; idx >= 0; idx-- {
		rowToDelete := toSheetCoord(rowsToDelete[idx] + rowOffset)
		err := f.RemoveRow(automatedSheet, rowToDelete)
		if err != nil {
			return fmt.Errorf("failed removing row %d from %s sheet in file %s: %s", rowToDelete, automatedSheet, input.Filename, err)
		}
	}

	err = f.Save()
	if err != nil {
		return fmt.Errorf("failed saving %s after synchronizing automated sheet users to MACFin users: %s", input.Filename, err)
	}

	return nil
}

func writeCell(f *excelize.File, sheet string, xCoord, yCoord int, value string) error {
	cellName, err := excelize.CoordinatesToCellName(toSheetCoord(xCoord), toSheetCoord(yCoord))
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
	srcCell, err := excelize.CoordinatesToCellName(toSheetCoord(srcX), toSheetCoord(srcY))
	if err != nil {
		return err
	}
	destCell, err := excelize.CoordinatesToCellName(toSheetCoord(destX), toSheetCoord(destY))
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
	userToPasswordRow, err := getManagedUsers(f, input)
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

	for i, row := range rows[rowOffset:] {
		if row == nil {
			continue
		}
		user := row[userX]
		macPassword := row[passwordX]
		if pwRow, ok := userToPasswordRow[user]; ok {
			if pwRow.Password != macPassword {
				err := writeCell(f, input.SheetName, passwordX, i+rowOffset, pwRow.Password)
				if err != nil {
					return fmt.Errorf("error setting new password for user %s in sheet %s in row %d: %s", user, input.SheetName, toSheetCoord(i+rowOffset), err)
				}
				numPasswordsUpdated++
			}
		} else {
			return fmt.Errorf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new passwords", user, input.SheetName)
		}
	}

	return nil
}
