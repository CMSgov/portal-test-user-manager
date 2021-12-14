package main

import (
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	thirtyDays  int = 30
	rowOffset   int = 1
	sheetOffset int = 1
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
	IDMHostname string // identity management hostname
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

func resetPasswords(f *excelize.File, config *Portal) (err error) {
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return err
	}

	var now time.Time

	numSuccess := 0
	numFail := 0
	numNoRotation := 0

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
		name := row[user]

		if row[timestamp] == "Rotate Now" {
			// force rotation
			lastRotated = now.AddDate(0, 0, -thirtyDays-2)
		} else {
			lastRotated, err = time.Parse(time.UnixDate, row[timestamp])
			if err != nil {
				return fmt.Errorf("error parsing timestamp from row %d for user %s: %s", i+rowOffset, name, err)
			}
		}

			log.Printf("%s: no rotation needed", row[colUser])
			numNoRotation++
			continue
		} else {
			newPassword := randomPasswords[i]
			err = changeUserPassword(client, config, name, row[portal], newPassword)
			if err != nil {
				numFail++
				log.Printf("Error: user %s password reset FAIL: %s", name, err)
				continue
			}
			numSuccess++
			// copy portal col password to previous col
			err = copyCell(f, config.Filename, portal+sheetOffset, i+rowOffset+sheetOffset, previous+sheetOffset, i+rowOffset+sheetOffset)
			if err != nil {
				return fmt.Errorf("failed to write previous password %s to sheet %s, row %d for user %s: %s",
					row[colPortal], input.SheetName, i+rowOffset, name, err)
			}

			// Write new password to portal col
			err = writeCell(f, config.Filename, automatedSheet, portal+sheetOffset, i+rowOffset+sheetOffset, newPassword)
			if err != nil {
				return fmt.Errorf("failed to write new password to sheet %s in row %d for user %s: %v; manually set password for user",
					input.SheetName, i+rowOffset+sheetOffset, name, err)
			}
			// set timestamp
			ts := now.Format(time.UnixDate)
			err = writeCell(f, config.Filename, automatedSheet, timestamp+sheetOffset, i+rowOffset+sheetOffset, ts)
			if err != nil {
				return fmt.Errorf("failed to write timestamp %s to sheet %s in row %d for user %s: %s", ts,
					input.SheetName, i+rowOffset+sheetOffset, name, err)
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
		IDMHostname: os.Getenv("IDMHOSTNAME"),
	}

	f, err := excelize.OpenFile(input.Filename)
	if err != nil {
		log.Fatal(err)
	}

	// true means "block action"
	err = f.ProtectSheet(automatedSheet, &excelize.FormatSheetProtection{
		Password:            portal.SheetPassword,
		SelectLockedCells:   true,
		SelectUnlockedCells: true,
	})
	if err != nil {
		log.Fatalf("failed to protect %s sheet", automatedSheet)
	}

	errors := validateFileSize(f, input)
	for _, err := range errors {
		log.Println(err)
	}
	if len(errors) > 0 {
		log.Fatal("Error: File size not supported. Exiting program.")
	}

	err = syncPasswordManagerUsersToMACFINUsers(f, portal)
	if err != nil {
		log.Fatal(err)
	}

	err = resetPasswords(f, portal)
	if err != nil {
		log.Fatal(err)
	}

	err = updateMacFinUsers(f, portal)
	if err != nil {
		log.Fatal(err)
	}
}
