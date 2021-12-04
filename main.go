package main

import (
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/xuri/excelize/v2"
)

type Input struct {
	Filename       string
	SheetName      string
	UsernameHeader string
	PasswordHeader string
	SheetPassword  string
}

type Portal struct {
	*Input
	Environment string
	Hostname    string
	IdmHostname string // identity management hostname
	infoLog     *log.Logger
	errorLog    *log.Logger
}

type Creds struct {
	OldPassword string
	NewPassword string
}

var client *http.Client

func init() {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal("error creating cookiejar")
	}
	client = &http.Client{
		Jar: jar,
	}
}

func resetPasswords(f *excelize.File, config *Portal) error {
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return err
	}

	var now time.Time

	rowOffset := 1   // row 0 is the header row
	sheetOffset := 1 // sheet starts counting from 1

	numSuccess := 0
	numFail := 0
	numNoRotation := 0

	var lastRotated time.Time

	for i, row := range rows[rowOffset:] {
		if row[timestamp] == "Rotate Now" {
			// force rotation
			ts, _ := time.Parse(time.UnixDate, row[timestamp])
			lastRotated = ts.AddDate(0, 0, -32)
		} else {
			lastRotated, _ = time.Parse(time.UnixDate, row[timestamp])
		}

		now = time.Now().UTC()

		name := row[user]

		if now.Before(lastRotated.AddDate(0, 0, 30)) {
			config.infoLog.Printf("%s: no rotation needed", row[user])
			numNoRotation++
			continue
		} else {
			newPassword := getRandomPassword(passwordLength)

			// write new password to local
			writeCell(f, automatedSheet, local+sheetOffset, i+rowOffset+sheetOffset, newPassword)
			// reset password in the portal
			err = changeUserPassword(client, config, name, row[portal], newPassword)
			if err != nil {
				numFail++
				config.errorLog.Printf("user %s password reset FAIL: %v", name, err)
				// write old password to local
				writeCell(f, automatedSheet, local+sheetOffset, i+rowOffset+sheetOffset, row[portal])
				continue
			}
			numSuccess++
			// copy portal password to previous
			copyCell(f, automatedSheet, portal+sheetOffset, i+rowOffset+sheetOffset, previous+sheetOffset, i+rowOffset+sheetOffset)
			// copy local password to portal
			copyCell(f, automatedSheet, local+sheetOffset, i+rowOffset+sheetOffset, portal+sheetOffset, i+rowOffset+sheetOffset)
			// set timestamp
			writeCell(f, automatedSheet, timestamp+sheetOffset, i+rowOffset+sheetOffset, time.Now().UTC().Format(time.UnixDate))
			// save file
			err = f.SaveAs(config.Filename)
			if err != nil {
				config.errorLog.Printf("failed to same to file for user %s", name)
				return err
			}
			config.infoLog.Printf("%s: rotation complete", row[user])
		}
	}

	config.infoLog.Printf("total rotations: %d success: %d  fail: %d  not rotated: %d total users: %d",
		numSuccess+numFail, numSuccess, numFail, numNoRotation, len(rows)-1)

	return nil
}

func main() {

	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime|log.Lshortfile)
	errorLog := log.New(os.Stderr, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	input := &Input{
		SheetName:      os.Getenv("SHEETNAME"),
		UsernameHeader: os.Getenv("USERNAMEHEADER"),
		PasswordHeader: os.Getenv("PASSWORDHEADER"),
		Filename:       os.Getenv("FILENAME"),
		SheetPassword:  os.Getenv("SHEETPASSWORD"),
	}

	portal := &Portal{
		Input:       input,
		Environment: os.Getenv("ENVIRONMENT"),
		Hostname:    os.Getenv("PORTALHOSTNAME"),
		IdmHostname: os.Getenv("IDMHOSTNAME"),
		infoLog:     infoLog,
		errorLog:    errorLog,
	}

	err := validateFilenameLength(input)
	if err != nil {
		errorLog.Fatal(err)
	}

	f, err := excelize.OpenFile(input.Filename)
	if err != nil {
		errorLog.Fatal(err)
	}

	// true means "block action"
	err = f.ProtectSheet(automatedSheet, &excelize.FormatSheetProtection{
		Password:            portal.SheetPassword,
		SelectLockedCells:   true,
		SelectUnlockedCells: true,
	})
	if err != nil {
		portal.errorLog.Fatalf("failed to protect %s sheet", automatedSheet)
	}

	errors := validateFileSize(f, input)
	for _, err := range errors {
		infoLog.Println(err)
	}
	if len(errors) > 0 {
		errorLog.Fatal("File size not supported. Exiting program.")
	}

	err = syncPasswordManagerUsersToMACFINUsers(f, portal)
	if err != nil {
		errorLog.Fatal(err)
	}

	err = f.ProtectSheet(automatedSheet, &excelize.FormatSheetProtection{
		Password: input.SheetPassword,
	})
	if err != nil {
		errorLog.Fatal(err)
	}

	err = resetPasswords(f, portal)
	if err != nil {
		errorLog.Fatal(err)
	}
}
