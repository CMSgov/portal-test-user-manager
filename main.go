package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path"
	"time"

	"github.com/xuri/excelize/v2"
)

type Column int
type Environment int

const (
	ColUser Column = iota
	ColPassword
	ColPrevious
	ColTimestamp
)

const (
	ColUserHeading      = "Username"
	ColPasswordHeading  = "Password"
	ColPreviousHeading  = "Previous"
	ColTimestampHeading = "Timestamp"
)

const (
	maxPasswordAgeDays int = 30
)

const (
	dev Environment = iota
	val
	prod
)

type Input struct {
	Bucket                         string
	Key                            string
	UsernameHeader                 string
	PasswordHeader                 string
	AutomatedSheetPassword         string
	AutomatedSheetColNameToIndex   map[Column]int
	AutomatedSheetColNameToHeading map[Column]string
	RowOffset                      int // number of header rows (common to all sheets)
	SheetGroups                    map[Environment]SheetGroup
}

type Portal struct {
	Hostname    string
	IDMHostname string // identity management hostname
	Scheme      string
}

type SheetGroup struct {
	AutomatedSheetName string // sheet managed by application
	SheetName          string
}

type Creds struct {
	OldPassword string
	NewPassword string
}

func portalClient() *http.Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatal("Error creating cookiejar")
	}
	return &http.Client{
		Jar: jar,
	}
}

func resetPasswords(f *excelize.File, input *Input, portal *Portal, s3Client S3ClientAPI, env Environment) (err error) {
	automatedSheet := input.SheetGroups[env].AutomatedSheetName
	rows, err := f.GetRows(automatedSheet)
	if err != nil {
		return err
	}

	sheetName := input.SheetGroups[env].SheetName
	mcFinUsersToPasswordRow, err := getMACFinUsers(f, input, env)
	if err != nil {
		return err
	}
	mfRows, err := f.GetRows(sheetName)
	if err != nil {
		return err
	}
	headerNameToXCoord := getHeaderToXCoord(mfRows[0])
	passwordXCoord := headerNameToXCoord[input.PasswordHeader]

	var now time.Time

	numSuccess := 0
	numFail := 0
	numNoRotation := 0
	rowOffset := input.RowOffset
	colUser := input.AutomatedSheetColNameToIndex[ColUser]
	colPassword := input.AutomatedSheetColNameToIndex[ColPassword]
	colPrevious := input.AutomatedSheetColNameToIndex[ColPrevious]
	colTimestamp := input.AutomatedSheetColNameToIndex[ColTimestamp]

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
		// delete all cookies
		client := portalClient()

		now = time.Now().UTC()
		name := row[colUser]

		if row[colTimestamp] == "Rotate Now" {
			// force rotation
			lastRotated = now.AddDate(0, 0, -maxPasswordAgeDays-1)
		} else {
			lastRotated, err = time.Parse(time.UnixDate, row[colTimestamp])
			if err != nil {
				return fmt.Errorf("Error parsing timestamp from row %d for user %s: %s", i+rowOffset, name, err)
			}
		}

		if now.Before(lastRotated.AddDate(0, 0, maxPasswordAgeDays)) {
			log.Printf("%s: no rotation needed", row[colUser])
			numNoRotation++
			continue
		} else {
			newPassword := randomPasswords[i]
			err = changeUserPassword(client, portal, name, row[colPassword], newPassword)
			if err != nil {
				numFail++
				log.Printf("Error: user %s password reset FAIL: %s", name, err)
				continue
			}
			numSuccess++
			// copy password to previous col
			err = copyCell(f, automatedSheet, colPassword, i+rowOffset, colPrevious, i+rowOffset)
			if err != nil {
				return fmt.Errorf("failed to write previous password %s to sheet %s, row %d for user %s: %s",
					row[colPassword], automatedSheet, i+rowOffset, name, err)
			}

			// write new password to password col
			err := writeCell(f, automatedSheet, colPassword, i+rowOffset, newPassword)
			if err != nil {
				return fmt.Errorf("failed to write new password to sheet %s in row %d for user %s: %v; manually set password for user",
					automatedSheet, toSheetCoord(i+rowOffset), name, err)
			}
			// set timestamp
			ts := now.Format(time.UnixDate)
			err = writeCell(f, automatedSheet, colTimestamp, i+rowOffset, ts)
			if err != nil {
				return fmt.Errorf("failed to write timestamp %s to sheet %s in row %d for user %s: %s", ts,
					automatedSheet, toSheetCoord(i+rowOffset), name, err)
			}

			log.Printf("%s: rotation complete", name)

			// update password for user in macFin sheet
			if pwRow, ok := mcFinUsersToPasswordRow[name]; !ok {
				return fmt.Errorf("macFin user %s missing from PasswordManager users; failed to update sheet %s with new password", name, sheetName)
			} else {
				err = writeCell(f, sheetName, passwordXCoord, pwRow.Row, newPassword)
				if err != nil {
					return fmt.Errorf("failed to write password for user %s to sheet %s in row %d: %s", name,
						sheetName, toSheetCoord(pwRow.Row), err)
				}
			}

			err = uploadFile(f, input.Bucket, input.Key, s3Client)
			if err != nil {
				return fmt.Errorf("Error uploading file after successful rotation: %s", err)
			}
			log.Printf("successfully uploaded file after rotating password for MACFin user %s", name)
		}
	}

	log.Printf("total rotations in %s: %d success: %d  fail: %d  not rotated: %d total users: %d",
		automatedSheet, numSuccess+numFail, numSuccess, numFail, numNoRotation, len(rows)-1)

	return nil
}

func rotate(input *Input, envToPortal map[Environment]*Portal, client S3ClientAPI) error {
	f, err := downloadFile(input, client)
	if err != nil {
		return err
	}
	defer os.RemoveAll(path.Dir(f.Path))

	err = validateSheets(f, input)
	if err != nil {
		return err
	}

	for env, portal := range envToPortal {
		// true means "block action"
		err = f.ProtectSheet(input.SheetGroups[env].AutomatedSheetName, &excelize.FormatSheetProtection{
			Password:            input.AutomatedSheetPassword,
			SelectLockedCells:   true,
			SelectUnlockedCells: true,
		})
		if err != nil {
			return fmt.Errorf("failed to protect %s sheet", input.SheetGroups[env].AutomatedSheetName)
		}

		err = syncPasswordManagerUsersToMACFinUsers(f, input, client, env)
		if err != nil {
			return err
		}

		err = resetPasswords(f, input, portal, client, env)
		if err != nil {
			return err
		}

		err = updateMACFinUsers(f, input, client, env)
		if err != nil {
			return err
		}

	}
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	envToPortal := map[Environment]*Portal{
		dev: {
			Hostname:    os.Getenv("PORTALHOSTNAMEDEV"),
			IDMHostname: os.Getenv("IDMHOSTNAMEDEV"),
			Scheme:      "https://",
		},
		val: {
			Hostname:    os.Getenv("PORTALHOSTNAMEVAL"),
			IDMHostname: os.Getenv("IDMHOSTNAMEVAL"),
			Scheme:      "https://",
		},
		prod: {
			Hostname:    os.Getenv("PORTALHOSTNAMEPROD"),
			IDMHostname: os.Getenv("IDMHOSTNAMEPROD"),
			Scheme:      "https://",
		},
	}

	input := &Input{
		UsernameHeader:         os.Getenv("USERNAMEHEADER"),
		PasswordHeader:         os.Getenv("PASSWORDHEADER"),
		Bucket:                 os.Getenv("BUCKET"),
		Key:                    os.Getenv("KEY"),
		AutomatedSheetPassword: os.Getenv("AUTOMATEDSHEETPASSWORD"),
		AutomatedSheetColNameToIndex: map[Column]int{
			ColUser: 0, ColPassword: 1, ColPrevious: 2, ColTimestamp: 3},
		AutomatedSheetColNameToHeading: map[Column]string{
			ColUser: ColUserHeading, ColPassword: ColPasswordHeading,
			ColPrevious: ColPreviousHeading, ColTimestamp: ColTimestampHeading},
		RowOffset: 1,
		SheetGroups: map[Environment]SheetGroup{
			dev: {
				AutomatedSheetName: "PasswordManager-DEV",
				SheetName:          os.Getenv("MACFINSHEETNAMEDEV"),
			},
			val: {
				AutomatedSheetName: "PasswordManager-VAL",
				SheetName:          os.Getenv("MACFINSHEETNAMEVAL"),
			},
			prod: {
				AutomatedSheetName: "PasswordManager-PROD",
				SheetName:          os.Getenv("MACFINSHEETNAMEPROD"),
			},
		},
	}

	client, err := createS3Client(region)
	if err != nil {
		log.Fatal(err)
	}

	err = rotate(input, envToPortal, client)
	if err != nil {
		log.Fatal(err)
	}
}
