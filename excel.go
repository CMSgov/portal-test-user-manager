package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

func contains(items []string, item string) bool {
	for _, it := range items {
		if it == item {
			return true
		}
	}

	return false
}

func validateSheetCols(f *excelize.File, input *Input, group SheetGroup, sheetName string) error {
	if sheetName == group.SheetName {
		// validate MACFin sheet cols
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return err
		}
		header := rows[0]
		// check for username header
		if !contains(header, input.UsernameHeader) {
			return fmt.Errorf("sheet %s in file s3://%s/%s does not contain header %s in top row", sheetName, input.Bucket, input.Key, input.UsernameHeader)
		}
		// check for password header
		if !contains(header, input.PasswordHeader) {
			return fmt.Errorf("sheet %s in file s3://%s/%s does not contain header %s in top row", sheetName, input.Bucket, input.Key, input.PasswordHeader)
		}
	} else if sheetName == group.AutomatedSheetName {
		// validate automated sheet cols
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return err
		}
		header := rows[0]
		// check number of cols
		if len(header) != len(input.AutomatedSheetColNameToIndex) {
			return fmt.Errorf("Expected sheet %s to have %d cols; it has %d cols", sheetName, len(input.AutomatedSheetColNameToIndex), len(header))
		}
		// check col headings and indexes
		for col, index := range input.AutomatedSheetColNameToIndex {
			if header[index] != input.AutomatedSheetColNameToHeading[col] {
				return fmt.Errorf("Expected %s in col %d; got %s", input.AutomatedSheetColNameToHeading[col], index, header[index])
			}
		}
	} else {
		return fmt.Errorf("Invalid sheet %s in file s3://%s/%s", sheetName, input.Bucket, input.Key)
	}
	return nil
}

func validateSheets(f *excelize.File, input *Input) error {
	sheetList := f.GetSheetList()
	for _, group := range input.SheetGroups {
		sheets := []string{group.SheetName, group.AutomatedSheetName}
		for _, sheet := range sheets {
			// check that sheet exists
			if !contains(sheetList, sheet) {
				return fmt.Errorf("sheet %s missing from file s3://%s/%s", sheet, input.Bucket, input.Key)
			}

			rows, err := f.GetRows(sheet)
			if err != nil {
				return err
			}

			// check if sheet is empty
			if len(rows) == 0 {
				return fmt.Errorf("sheet %s in file s3://%s/%s is empty; sheet must include header row", sheet, input.Bucket, input.Key)
			}
			// validate sheet columns
			err = validateSheetCols(f, input, group, sheet)
			if err != nil {
				return err
			}
		}
	}
	return nil
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

func deleteRows(f *excelize.File, sheet string, rowsToDelete []int) error {
	sort.Ints(rowsToDelete)
	for idx := len(rowsToDelete) - 1; idx >= 0; idx-- {
		rowToDelete := toSheetCoord(rowsToDelete[idx])
		err := f.RemoveRow(sheet, rowToDelete)
		if err != nil {
			return fmt.Errorf("failed removing row %d from %s sheet in file %s: %s", rowToDelete, sheet, f.Path, err)
		}
	}

	err := f.Save()
	if err != nil {
		return fmt.Errorf("failed saving file %s after synchronizing automated sheet users to MACFin users: %s", f.Path, err)
	}

	return nil
}

// Remove duplicate MACFin usernames and write usernames to lowercase
func removeMACFinUserDups(f *excelize.File, input *Input, env Environment) error {
	sheetName := input.SheetGroups[env].SheetName
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed getting rows from %s in s3://%s/%s: %s", sheetName, input.Bucket, input.Key, err)
	}

	headerToXCoord := getHeaderToXCoord(rows[0])
	usernameXCoord := headerToXCoord[input.UsernameHeader]
	passwordXCoord := headerToXCoord[input.PasswordHeader]
	rowOffset := input.RowOffset

	users := make(map[string]bool, len(rows)-rowOffset)
	rowsToDelete := []int{}
	for idx := len(rows) - 1; idx >= rowOffset; idx-- {
		err := validateRow(f, sheetName, idx, usernameXCoord, passwordXCoord)
		if err != nil {
			log.Printf("Info: validating sheet %s, row %d: %s", sheetName, toSheetCoord(idx), err)
			continue
		}
		if _, ok := users[strings.ToLower(rows[idx][usernameXCoord])]; ok {
			rowsToDelete = append(rowsToDelete, idx)
		} else {
			users[strings.ToLower(rows[idx][usernameXCoord])] = true
			writeCell(f, sheetName, usernameXCoord, idx, strings.ToLower(rows[idx][usernameXCoord]))
		}
	}

	if len(rowsToDelete) > 0 {
		err = deleteRows(f, sheetName, rowsToDelete)
		if err != nil {
			return fmt.Errorf("Error deleting duplicate users from sheet %s in file s3://%s/%s:%s", sheetName, input.Bucket, input.Key, err)
		}
		log.Printf("successfully deleted all duplicate users (%d) from %s in file s3://%s/%s", len(rowsToDelete), sheetName, input.Bucket, input.Key)
	}

	return nil
}

func removeDupsFromMACFinSheets(f *excelize.File, input *Input) error {
	for env := range input.SheetGroups {
		err := removeMACFinUserDups(f, input, env)
		if err != nil {
			return err
		}
	}
	return nil
}

func getMACFinUsers(f *excelize.File, input *Input, env Environment) (map[string]PasswordRow, error) {
	sheetName := input.SheetGroups[env].SheetName
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed getting rows from %s in s3://%s/%s: %s", sheetName, input.Bucket, input.Key, err)
	}

	users := make(map[string]PasswordRow)

	headerToXCoord := getHeaderToXCoord(rows[0])
	usernameXCoord := headerToXCoord[input.UsernameHeader]
	passwordXCoord := headerToXCoord[input.PasswordHeader]
	rowOffset := input.RowOffset

	for i, row := range rows[rowOffset:] {
		err := validateRow(f, sheetName, i+rowOffset, usernameXCoord, passwordXCoord)
		if err != nil {
			log.Printf("Info: validating sheet %s, row %d: %s", sheetName, toSheetCoord(i+rowOffset), err)
			continue
		}
		users[strings.ToLower(row[usernameXCoord])] = PasswordRow{row[passwordXCoord], i + rowOffset}
	}

	return users, nil
}

func getManagedUsers(f *excelize.File, input *Input, env Environment) (map[string]PasswordRow, error) {
	automatedSheet := input.SheetGroups[env].AutomatedSheetName
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
			Row:      i + rowOffset,
		}
	}

	return userToPasswordRow, nil
}

// Sync PasswordManager usernames with MACFin users
func syncPasswordManagerUsersToMACFinUsers(f *excelize.File, input *Input, client S3ClientAPI, env Environment) error {
	macFinUsersToPasswordRow, err := getMACFinUsers(f, input, env)
	if err != nil {
		return err
	}

	userToPasswordRow, err := getManagedUsers(f, input, env)
	if err != nil {
		return err
	}

	rowOffset := input.RowOffset
	initialNumRows := len(userToPasswordRow) + rowOffset
	numRows := initialNumRows
	automatedSheet := input.SheetGroups[env].AutomatedSheetName

	// add new MACFin users to automatedSheet
	for mfUser, up := range macFinUsersToPasswordRow {
		if _, ok := userToPasswordRow[mfUser]; !ok {
			values := map[Column]string{
				ColUser:      mfUser,
				ColPassword:  up.Password,
				ColPrevious:  up.Password,
				ColTimestamp: "Rotate Now",
			}
			// write new row
			for name, idx := range input.AutomatedSheetColNameToIndex {
				err := writeCell(f, automatedSheet, idx, numRows, values[name])
				if err != nil {
					return fmt.Errorf("failed adding new MACFin user %s to %s sheet in file %s: %s", mfUser, automatedSheet, f.Path, err)
				}
			}
			numRows++
			continue
		}
	}

	// get rows for deletion from automatedSheet
	rowsToDelete := []int{}
	for pwUser, pwRow := range userToPasswordRow {
		if _, ok := macFinUsersToPasswordRow[pwUser]; !ok {
			// pwUser is not in MACFin users; mark row for deletion from automatedSheet
			rowsToDelete = append(rowsToDelete, pwRow.Row)
			continue
		}
	}

	// delete rows from automatedSheet that are marked for deletion
	if len(rowsToDelete) > 0 {
		err = deleteRows(f, automatedSheet, rowsToDelete)
		if err != nil {
			return err
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

	err = uploadFile(f, input.Bucket, input.Key, client)
	if err != nil {
		return fmt.Errorf("Error uploading file after synchronizing: %s", err)
	}
	log.Printf("successfully uploaded file to s3://%s/%s after syncrhonization", input.Bucket, input.Key)

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
func updateMACFinUsers(f *excelize.File, input *Input, client S3ClientAPI, env Environment) error {
	userToPasswordRow, err := getManagedUsers(f, input, env)
	if err != nil {
		return err
	}

	sheetName := input.SheetGroups[env].SheetName
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return err
	}

	headerNameToXCoord := getHeaderToXCoord(rows[0])
	userX := headerNameToXCoord[input.UsernameHeader]
	passwordX := headerNameToXCoord[input.PasswordHeader]
	rowOffset := input.RowOffset

	for i, row := range rows[rowOffset:] {
		err := validateRow(f, sheetName, i+rowOffset, userX, passwordX)
		if err != nil {
			log.Printf("Info: validating sheet %s, row %d: %s", sheetName, toSheetCoord(i+rowOffset), err)
			continue
		}

		user := row[userX]
		macPassword := row[passwordX]
		if pwRow, ok := userToPasswordRow[strings.ToLower(user)]; ok {
			if pwRow.Password != macPassword {
				err := writeCell(f, sheetName, passwordX, i+rowOffset, pwRow.Password)
				if err != nil {
					return fmt.Errorf("Error setting new password for user %s in sheet %s in row %d: %s", user, sheetName, toSheetCoord(i+rowOffset), err)
				}
			}
		} else {
			return fmt.Errorf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new passwords", user, sheetName)
		}
	}

	err = uploadFile(f, input.Bucket, input.Key, client)
	if err != nil {
		return fmt.Errorf("Error uploading file: %s", err)
	}
	log.Printf("successfully uploaded file to s3://%s/%s after processing sheet %s", input.Bucket, input.Key, sheetName)

	return nil
}

// password-protect excel file
func protectExcelWorkbook(filename string) (string, error) {
	dir := filepath.Dir(filename)
	outFilename := filepath.Join(dir, "protected-"+filepath.Base(filename))

	cmdPath, err := exec.LookPath("secure-spreadsheet")
	if err != nil {
		return "", fmt.Errorf("Error resolving command name: %s", err)
	}
	secureSpreadsheet := exec.Command(cmdPath, "--password", os.Getenv("WORKBOOKPASSWORD"), "--input-format", "xlsx")

	inFile, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("Error opening excel file: %s err: %s", filename, err)
	}
	defer inFile.Close()

	outFile, err := os.Create(outFilename)
	if err != nil {
		return "", fmt.Errorf("Error creating output excel file: %s err: %s", outFilename, err)
	}
	defer outFile.Close()

	secureSpreadsheet.Stdin = inFile
	secureSpreadsheet.Stdout = outFile

	if err := secureSpreadsheet.Run(); err != nil {
		return "", fmt.Errorf("Error starting secure-spreadsheet command: %s", err)
	}

	log.Printf("Successfully protected workbook file: %s", outFilename)

	return outFilename, nil

}
