package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

type PasswordRow struct {
	Password string
	Row      int
}

func toSheetCoord(coord int) int {
	return coord + 1
}

func validateRow(f *excelize.File, sheet string, rowNum, usernameXCoord, passwordXCoord int) error {
	// check if username or password is empty
	username, err := getCellValue(f, sheet, usernameXCoord, rowNum)
	if err != nil {
		return err
	}
	if username == "" {
		return errors.New("username is missing")
	}
	password, err := getCellValue(f, sheet, passwordXCoord, rowNum)
	if err != nil {
		return err
	}

	if password == "" {
		return errors.New("password is missing")
	}

	return nil
}

func getHeaderToXCoord(headerRow []string) map[string]int {
	headerToXCoord := make(map[string]int, len(headerRow))
	for i, cell := range headerRow {
		headerToXCoord[cell] = i
	}
	return headerToXCoord
}

type usernameAndPassword struct {
	Username, Password string
}

func getMACFinUsers(f *excelize.File, input *Input) ([]usernameAndPassword, error) {
	rows, err := f.GetRows(input.SheetName)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in s3://%s/%s: %s", input.SheetName, input.Bucket, input.Key, err)
	}

	users := []usernameAndPassword{}

	headerToXCoord := getHeaderToXCoord(rows[0])
	usernameXCoord := headerToXCoord[input.UsernameHeader]
	passwordXCoord := headerToXCoord[input.PasswordHeader]
	rowOffset := input.RowOffset

	for i, row := range rows[rowOffset:] {
		err := validateRow(f, input.SheetName, i+rowOffset, usernameXCoord, passwordXCoord)
		if err != nil {
			log.Printf("validating sheet %s, row %d: %s", input.SheetName, toSheetCoord(i+rowOffset), err)
			continue
		}
		users = append(users, usernameAndPassword{strings.ToLower(row[usernameXCoord]), row[passwordXCoord]})
	}

	return users, nil
}

func getManagedUsers(f *excelize.File, input *Input) (map[string]PasswordRow, error) {
	automatedSheet := input.AutomatedSheetName
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in s3://%s/%s: %s", automatedSheet, input.Bucket, input.Key, err)
	}

	rowOffset := input.RowOffset
	colUser := input.AutomatedSheetColNameToIndex[ColUser]
	colPassword := input.AutomatedSheetColNameToIndex[ColPassword]
	userToPasswordRow := make(map[string]PasswordRow)

	for i, row := range rows[rowOffset:] {
		userToPasswordRow[row[colUser]] = PasswordRow{
			Password: row[colPassword],
			Row:      i,
		}
	}

	return userToPasswordRow, nil
}

// Sync PasswordManager usernames with MACFin users
func syncPasswordManagerUsersToMACFinUsers(f *excelize.File, input *Input, client S3ClientAPI) error {
	macFinUsersToPasswords, err := getMACFinUsers(f, input)
	if err != nil {
		return err
	}

	userToPasswordRow, err := getManagedUsers(f, input)
	if err != nil {
		return err
	}

	rowOffset := input.RowOffset
	initialNumRows := len(userToPasswordRow) + rowOffset
	numRows := initialNumRows
	automatedSheet := input.AutomatedSheetName

	usersFound := map[string]struct{}{}
	// add new MACFin users to automatedSheet
	for _, up := range macFinUsersToPasswords {
		if _, ok := usersFound[up.Username]; ok {
			continue
		}
		usersFound[up.Username] = struct{}{}
		if _, ok := userToPasswordRow[up.Username]; !ok {
			values := map[Column]string{
				ColUser:      up.Username,
				ColPassword:  up.Password,
				ColPrevious:  up.Password,
				ColTimestamp: "Rotate Now",
			}
			// write new row
			for name, idx := range input.AutomatedSheetColNameToIndex {
				err := writeCell(f, automatedSheet, idx, numRows, values[name])
				if err != nil {
					return fmt.Errorf("failed adding new MACFin user %s to %s sheet in file %s: %s", up.Username, automatedSheet, f.Path, err)
				}
			}
			numRows++
			continue
		}
	}

	// get rows for deletion from automatedSheet
	rowsToDelete := []int{}
	for pwUser, pwRow := range userToPasswordRow {
		if _, ok := usersFound[pwUser]; !ok {
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
			return fmt.Errorf("failed removing row %d from %s sheet in file %s: %s", rowToDelete, automatedSheet, f.Path, err)
		}
	}

	err = f.Save()
	if err != nil {
		return fmt.Errorf("failed saving file %s after synchronizing automated sheet users to MACFin users: %s", f.Path, err)
	}

	err = sortRows(f, input, automatedSheet)
	if err != nil {
		return fmt.Errorf("failed sorting %s after synchronizing sheet to MACFin users: %s", automatedSheet, err)
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

func getCellValue(f *excelize.File, sheet string, xCoord, yCoord int) (string, error) {
	cellName, err := excelize.CoordinatesToCellName(toSheetCoord(xCoord), toSheetCoord(yCoord))
	if err != nil {
		return "", err
	}
	value, err := f.GetCellValue(sheet, cellName)
	if err != nil {
		return "", err
	}
	return value, nil
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

func sortRows(f *excelize.File, input *Input, sheetname string) error {
	rows, err := f.GetRows(sheetname)
	if err != nil {
		return err
	}

	colUser := input.AutomatedSheetColNameToIndex[ColUser]

	sort.Slice(rows[input.RowOffset:], func(i, j int) bool {
		return rows[input.RowOffset+i][colUser] < rows[input.RowOffset+j][colUser]
	})

	// write sorted rows to automatedSheet
	for idx, r := range rows[input.RowOffset:] {
		cellName := fmt.Sprintf("A%d", 2+idx)
		err = f.SetSheetRow(sheetname, cellName, &r)
		if err != nil {
			return fmt.Errorf("Error writing sorted sheet: %s", err)
		}
	}

	err = f.Save()
	if err != nil {
		return fmt.Errorf("Error saving sheet %s: %s", sheetname, err)
	}
	return nil
}

// Write new password to password column in the MACFin sheet
func updateMACFinUsers(f *excelize.File, input *Input, client S3ClientAPI) error {
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
	rowOffset := input.RowOffset

	for i, row := range rows[rowOffset:] {
		err := validateRow(f, input.SheetName, i+rowOffset, userX, passwordX)
		if err != nil {
			log.Printf("validating sheet %s, row %d: %s", input.SheetName, toSheetCoord(i+rowOffset), err)
			continue
		}

		user := row[userX]
		macPassword := row[passwordX]
		if pwRow, ok := userToPasswordRow[strings.ToLower(user)]; ok {
			if pwRow.Password != macPassword {
				err := writeCell(f, input.SheetName, passwordX, i+rowOffset, pwRow.Password)
				if err != nil {
					return fmt.Errorf("Error setting new password for user %s in sheet %s in row %d: %s", user, input.SheetName, toSheetCoord(i+rowOffset), err)
				}
				numPasswordsUpdated++
			}
		} else {
			return fmt.Errorf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new passwords", user, input.SheetName)
		}
	}

	err = uploadFile(f, input.Bucket, input.Key, client)
	if err != nil {
		return fmt.Errorf("Error uploading file: %s", err)
	}
	log.Printf("successfully uploaded file to s3://%s/%s after updating sheet %s", input.Bucket, input.Key, input.SheetName)

	return nil
}
