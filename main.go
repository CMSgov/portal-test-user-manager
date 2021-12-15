package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	maxPasswordAgeDays int = 30
)

type Input struct {
	Filename                     string
	SheetName                    string
	UsernameHeader               string
	PasswordHeader               string
	AutomatedSheetPassword       string
	AutomatedSheetName           string // sheet managed by application
	AutomatedSheetColNameToIndex map[string]int
	RowOffset                    int // number of header rows (common to all sheets)
}

type Portal struct {
	Hostname    string
	IDMHostname string // identity management hostname
}

type Creds struct {
	OldPassword string
	NewPassword string
}

func portalClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal("error creating cookiejar")
	}
	return &http.Client{
		Jar: jar,
	}
}

func resetPasswords(f *excelize.File, input *Input, portal *Portal) (err error) {
	automatedSheet := input.AutomatedSheetName
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return err
	}

	client := portalClient()

	var now time.Time

	numSuccess := 0
	numFail := 0
	numNoRotation := 0
	rowOffset := input.RowOffset
	colUser := input.AutomatedSheetColNameToIndex["colUser"]
	colPortal := input.AutomatedSheetColNameToIndex["colPortal"]
	colPrevious := input.AutomatedSheetColNameToIndex["colPrevious"]
	colTimestamp := input.AutomatedSheetColNameToIndex["colTimestamp"]

	var lastRotated time.Time

	randomPasswords := make([]string, len(rows)-rowOffset)
	for i := 0; i < len(rows)-rowOffset; i++ {
		password, err := getRandomPassword()
		if err != nil {
			return err
		}
		randomPasswords[i] = password
	}

	for i, row := range rows[rowOffset:] {
		now = time.Now().UTC()
		name := row[colUser]

		if row[colTimestamp] == "Rotate Now" {
			// force rotation
			lastRotated = now.AddDate(0, 0, -maxPasswordAgeDays-1)
		} else {
			lastRotated, err = time.Parse(time.UnixDate, row[colTimestamp])
			if err != nil {
				return fmt.Errorf("error parsing timestamp from row %d for user %s: %s", i+rowOffset, name, err)
			}
		}

		if now.Before(lastRotated.AddDate(0, 0, maxPasswordAgeDays)) {
			log.Printf("%s: no rotation needed", row[colUser])
			numNoRotation++
			continue
		} else {
			newPassword := randomPasswords[i]
			err = changeUserPassword(client, portal, name, row[colPortal], newPassword)
			if err != nil {
				numFail++
				log.Printf("Error: user %s password reset FAIL: %s", name, err)
				continue
			}
			numSuccess++
			// copy portal col password to previous col
			err = copyCell(f, automatedSheet, colPortal, i+rowOffset, colPrevious, i+rowOffset)
			if err != nil {
				return fmt.Errorf("failed to write previous password %s to sheet %s, row %d for user %s: %s",
					row[colPortal], input.SheetName, i+rowOffset, name, err)
			}

			// Write new password to portal col
			err := writeCell(f, automatedSheet, colPortal, i+rowOffset, newPassword)
			if err != nil {
				return fmt.Errorf("failed to write new password to sheet %s in row %d for user %s: %v; manually set password for user",
					input.SheetName, toSheetCoord(i+rowOffset), name, err)
			}
			// set timestamp
			ts := now.Format(time.UnixDate)
			err = writeCell(f, automatedSheet, colTimestamp, i+rowOffset, ts)
			if err != nil {
				return fmt.Errorf("failed to write timestamp %s to sheet %s in row %d for user %s: %s", ts,
					input.SheetName, toSheetCoord(i+rowOffset), name, err)
			}

			log.Printf("%s: rotation complete", row[colUser])
		}
	}

	log.Printf("total rotations: %d success: %d  fail: %d  not rotated: %d total users: %d",
		numSuccess+numFail, numSuccess, numFail, numNoRotation, len(rows)-1)

	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	input := &Input{
		SheetName:              os.Getenv("MACFINSHEETNAME"),
		UsernameHeader:         os.Getenv("USERNAMEHEADER"),
		PasswordHeader:         os.Getenv("PASSWORDHEADER"),
		Filename:               os.Getenv("FILENAME"),
		AutomatedSheetPassword: os.Getenv("AUTOMATEDSHEETPASSWORD"),
		AutomatedSheetName:     "PasswordManager",
		AutomatedSheetColNameToIndex: map[string]int{
			"colUser": 0, "colPortal": 1, "colPrevious": 2, "colTimestamp": 3},
		RowOffset: 1,
	}

	portal := &Portal{
		Hostname:    os.Getenv("PORTALHOSTNAME"),
		IDMHostname: os.Getenv("IDMHOSTNAME"),
	}

	f, err := excelize.OpenFile(input.Filename)
	if err != nil {
		log.Fatal(err)
	}

	// true means "block action"
	err = f.ProtectSheet(input.AutomatedSheetName, &excelize.FormatSheetProtection{
		Password:            input.AutomatedSheetPassword,
		SelectLockedCells:   true,
		SelectUnlockedCells: true,
	})
	if err != nil {
		log.Fatalf("failed to protect %s sheet", input.AutomatedSheetName)
	}

	errors := validateFileSize(f, input)
	for _, err := range errors {
		log.Println(err)
	}
	if len(errors) > 0 {
		log.Fatal("Error: File size not supported. Exiting program.")
	}

	err = syncPasswordManagerUsersToMACFinUsers(f, input)
	if err != nil {
		log.Fatal(err)
	}

	err = resetPasswords(f, input, portal)
	if err != nil {
		log.Fatal(err)
	}

	err = updateMACFinUsers(f, input)
	if err != nil {
		log.Fatal(err)
	}
}
