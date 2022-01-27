package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/xuri/excelize/v2"
)

var now = time.Now()

func format(d time.Duration) string {
	return now.Add(d).Format(time.UnixDate)
}

const newPasswordMarker = "newPassword"

const (
	sheetNameMACFin       = "MACFin"
	headingMACFinUsername = "User"
	headingMACFinPassword = "Password"

	sheetNamePasswordManager = "PasswordManager"
	inputBucket              = "macfin"
	inputKey                 = "pre1/pre2/macfin-dev-lbk-s3.xlsx"
)

func cn(col, row int) string {
	name, err := excelize.CoordinatesToCellName(col, row, false)
	if err != nil {
		panic(err)
	}
	return name
}

func verifyNewPasswordIsUploaded(fc *FakeS3Client, contents []byte) error {
	existingFp, err := excelize.OpenFile(fc.LocalPath)
	if err != nil {
		return err
	}

	newFp, err := excelize.OpenReader(bytes.NewReader(contents))
	if err != nil {
		return err
	}

	oldAutomatedRows, err := existingFp.GetRows(fc.AutomatedSheetName)
	if err != nil {
		return err
	}
	newMACFinRows, err := newFp.GetRows(fc.SheetName)
	if err != nil {
		return err
	}
	newAutomatedRows, err := newFp.GetRows(fc.AutomatedSheetName)
	if err != nil {
		return err
	}

	colUser := fc.AutomatedSheetColNameToIndex[ColUser]
	colPassword := fc.AutomatedSheetColNameToIndex[ColPassword]
	colPrevious := fc.AutomatedSheetColNameToIndex[ColPrevious]
	oldUserToPassword := make(map[string]string, 0)
	for _, upr := range oldAutomatedRows[fc.RowOffset:] {
		oldUserToPassword[strings.ToLower(upr[colUser])] = upr[colPassword]
	}

	numNewPasswords := 0

	headerToXCoord := getHeaderToXCoord(newMACFinRows[0])
	for _, cr := range newAutomatedRows[fc.RowOffset:] {
		user := strings.ToLower(cr[colUser])
		password := cr[colPassword]
		passwordChanged := false
		if pw, ok := oldUserToPassword[user]; !ok {
			// new user with new password
			if password != cr[colPrevious] {
				passwordChanged = true
			}
		} else if pw != cr[colPassword] {
			// new password for existing user
			passwordChanged = true
		}
		if passwordChanged {
			numNewPasswords++
			// MACFin sheet should be updated with new password
			for _, row := range newMACFinRows[fc.RowOffset:] {
				if len(row) < headerToXCoord[fc.UsernameHeader] || len(row) < headerToXCoord[fc.PasswordHeader] {
					continue // short, invalid row
				}
				if strings.ToLower(row[headerToXCoord[fc.UsernameHeader]]) == user {
					if password != row[headerToXCoord[fc.PasswordHeader]] {
						return fmt.Errorf("MACFin sheet not updated in %s after a password rotation", fc.LocalPath)
					}
				}
			}
		}
	}
	if numNewPasswords > 1 {
		return fmt.Errorf("Skipped file upload to %s after a password rotation", fc.LocalPath)
	}

	return nil
}

const (
	portalSessionCookieName = "IDMSession"
	idmSessionCookieName    = "jsession"
	xsrfCookieName          = "PORTAL-XSRF-TOKEN"
)

const (
	// Use two names for the same host so that cookies are not shared
	portalServer = "127.0.0.1:3398"
	idmServer    = "localhost:3398"
)

const (
	// loginOauth2Path redirects to this to set the XSRF cookie. This does
	// not match the real-life path but we want the client to just follow
	// whatever the redirect is.
	xsrfRedirectPath = "/portal/set-xsrf"
)

type currentFilePath string

type session struct {
	username  string
	token     string
	xsrfToken string
	loggedOut bool
}

type AuthServer struct {
	UserToPassword    map[string]string
	UserToNewPassword map[string]string
	Errors            map[string]string // user -> path
	PortalSessions    map[string]*session
	IDMSessions       map[string]*session
}

func (s *AuthServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.PortalSessions == nil {
		s.PortalSessions = make(map[string]*session)
		s.IDMSessions = make(map[string]*session)
		if s.Errors == nil {
			s.Errors = make(map[string]string)
		}
	}
	log.Printf("Got a request for %s?%s with cookies %v and host %v", r.URL.Path, r.URL.RawQuery, r.Cookies(), r.Host)
	// IDMS paths
	if !strings.HasPrefix(r.URL.Path, "/portal/") && !strings.HasPrefix(r.URL.Path, "/myportal/") {
		if r.URL.Path == loginOauth2Path {
			var sess *session
			tok := r.URL.Query().Get("token")
			for _, s := range s.PortalSessions {
				if s.token == tok {
					sess = s
					break
				}
			}
			if sess == nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}
			if s.Errors[sess.username] == r.URL.Path {
				http.Error(w, "Error triggered for test", http.StatusInternalServerError)
				return
			}
			sessionID := fmt.Sprintf("%x", rand.Int())
			s.IDMSessions[sessionID] = sess
			w.Header().Set("Set-Cookie", idmSessionCookieName+"="+sessionID+"; Path=/")
			http.Redirect(w, r, "http://"+portalServer+xsrfRedirectPath, http.StatusFound)
		} else {
			http.Error(w, "Unsupported IDM path: "+r.URL.Path, http.StatusInternalServerError)
		}
		return
	}
	// Portal paths
	if r.URL.Path == loginClearPath {
		sessionID := fmt.Sprintf("%x", rand.Int())
		s.PortalSessions[sessionID] = &session{}
		w.Header().Set("Set-Cookie", portalSessionCookieName+"="+sessionID+"; Path=/")
	} else if cookie, err := r.Cookie(portalSessionCookieName); err != nil {
		http.Error(w, "No Portal cookie", http.StatusUnauthorized)
		return
	} else if sess, ok := s.PortalSessions[cookie.Value]; !ok {
		http.Error(w, "Bad Portal cookie", http.StatusUnauthorized)
		return
	} else if sess.loggedOut {
		http.Error(w, "Using a logged-out session", http.StatusUnauthorized)
		return
	} else {
		if r.URL.Path == loginSubmitPath {
			ld := &loginDetails{}
			err := json.NewDecoder(r.Body).Decode(ld)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error decoding login details: %s", err), http.StatusBadRequest)
				return
			}
			if password, ok := s.UserToPassword[ld.Username]; !ok {
				http.Error(w, fmt.Sprintf("Bad user %q", ld.Username), http.StatusForbidden)
				return
			} else if password != ld.Password {
				http.Error(w, fmt.Sprintf("Bad password for %q: %q != %q", ld.Username, password, ld.Password), http.StatusForbidden)
				return
			}
			ud := userData{
				SessionToken: "st-" + cookie.Value,
			}
			sess.username = ld.Username
			if s.Errors[sess.username] == r.URL.Path {
				http.Error(w, "Error triggered for test", http.StatusInternalServerError)
				return
			}
			sess.token = ud.SessionToken
			err = json.NewEncoder(w).Encode(ud)
			if err != nil {
				panic(err)
			}
		} else if r.URL.Path == changePasswordPath {
			if s.Errors[sess.username] == r.URL.Path {
				http.Error(w, "Error triggered for test", http.StatusInternalServerError)
				return
			}
			cp := &changePassword{}
			err := json.NewDecoder(r.Body).Decode(cp)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error decoding password change details: %s", err), http.StatusBadRequest)
				return
			}
			if cp.OldPassword != s.UserToPassword[sess.username] {
				http.Error(w, "Incorrect old password", http.StatusBadRequest)
				return
			}
			if len(cp.NewPassword) != passwordLength {
				http.Error(w, "Invalid password length", http.StatusBadRequest)
				return
			}
			for _, class := range []string{digits, uppers, lowers, specials} {
				if !strings.ContainsAny(cp.NewPassword, class) {
					http.Error(w, fmt.Sprintf("Password needs a character from %q", class), http.StatusBadRequest)
					return
				}
			}
			if s.UserToNewPassword == nil {
				s.UserToNewPassword = make(map[string]string)
			}
			s.UserToNewPassword[sess.username] = cp.NewPassword
			s.UserToPassword[sess.username] = cp.NewPassword
		} else if r.URL.Path == xsrfRedirectPath {
			tok := fmt.Sprintf("%x", rand.Int())
			sess.xsrfToken = tok
			w.Header().Set("Set-Cookie", xsrfCookieName+"="+tok+"; Path=/")
		} else if r.URL.Path == logoutPath {
			sess.loggedOut = true
		} else {
			http.Error(w, "Unsupported Portal path: "+r.URL.Path, http.StatusInternalServerError)
		}
	}
}

const Day = time.Hour * 24

type PasswordManagerRow struct {
	Username  string
	Password  string
	Previous  string
	Timestamp time.Duration
}

type MACFinRow struct {
	Username string
	Password string
}

type SheetProblem int

const (
	NoSheetProblem SheetProblem = iota
	SheetProblemInvalidMACFinUsernameHeading
	SheetProblemInvalidMACFinPasswordHeading
	SheetProblemMACFinEmpty
	SheetProblemPasswordManagerEmpty
	SheetProblemPasswordManagerTooManyHeadings
	SheetProblemPasswordManagerTooFewHeadings
	SheetProblemPasswordManagerInvalidHeadingOrder
)

type TestCase struct {
	Name               string
	PasswordManagerIn  []PasswordManagerRow
	PasswordManagerOut []PasswordManagerRow
	MACFinIn           []MACFinRow
	UntrackedPasswords map[string]string // user -> password
	ServerErrors       map[string]string // user -> path
	SheetInProblem     SheetProblem
}

var testCases = []TestCase{
	{
		Name: "rotate some",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
			{
				"james", "baz", "", -20 * Day,
			},
			{
				"chuck", "y", "", -1000 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"ben", newPasswordMarker, "x", 0,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
			{
				"chuck", newPasswordMarker, "y", 0,
			},
			{
				"james", "baz", "", -20 * Day,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
			{"leslie", "bar"},
			{"james", "baz"},
			{"ben", "x"},
			{"", ""}, // make sure we aren't confused by blank entries
			{"chuck", "y"},
			{"leslie", "bar"}, // repeated
			{"LESLIe", "bar"}, // repeated with different capitalization
		},
	},
	{
		Name: "delete at end",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"ben", newPasswordMarker, "x", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"ben", "x"},
		},
	},
	{
		Name: "delete middle",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
			{
				"chuck", "y", "", 20 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"ben", newPasswordMarker, "x", 0,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"ben", "x"},
			{"Leslie", "bar"}, // uppercase
		},
	},
	{
		Name: "delete at beginning",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", "foo", "", 20 * Day,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
		},
	},
	{
		Name: "delete all",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
	},
	{
		Name: "add new",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
			{
				"james", newPasswordMarker, "baz", 0,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"james", "baz2"},
			{"james", "baz"}, // duplicate with different passwords
			{"chris", "foo"},
			{"leslie", "bar"},
			{"CHRIS", "foo"}, // duplicate with different capitalization
		},
		UntrackedPasswords: map[string]string{
			"leslie": "bar",
			"james":  "baz",
		},
	},
	{
		Name:              "add all",
		PasswordManagerIn: []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", newPasswordMarker, "foo", 0,
			},
			{
				"james", newPasswordMarker, "baz", 0,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"james", "baz"},
			{"chris", "foo2"},
			{"chris", "foo"},  // duplicate with different password
			{"Leslie", "bar"}, // contains uppercase
		},
		UntrackedPasswords: map[string]string{
			"Leslie": "bar",
			"james":  "baz",
			"chris":  "foo",
		},
	},
	{
		Name: "add and delete",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
			{
				"chris", "foo", "", 20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", "foo", "", 20 * Day,
			},
			{
				"chuck", newPasswordMarker, "chuckie", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
			{"CHUCK", "chuckie2"},
			{"chuck", "chuckie"}, // duplicate with different password
		},
		UntrackedPasswords: map[string]string{
			"CHUCK": "chuckie",
		},
	},
	{
		Name: "error when logging in",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", -80 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", -80 * Day,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
			{"leslie", "bar"},
			{"ben", "x"},
		},
		ServerErrors: map[string]string{
			"chris": loginSubmitPath,
			"ben":   loginOauth2Path,
		},
	},
	{
		Name: "error when changing password",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"ben", "x", "", -80 * Day,
			},
			{
				"chris", "foo", "", -80 * Day,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"ben", newPasswordMarker, "x", 0,
			},
			{
				"chris", newPasswordMarker, "foo", 0,
			},
			{
				"leslie", "bar", "", -90 * Day,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
			{"leslie", "bar"},
			{"ben", "x"},
		},
		ServerErrors: map[string]string{
			"leslie": changePasswordPath,
		},
	},
	{
		Name: "wrong MACFinIn Username Heading",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
		},
		SheetInProblem: SheetProblemInvalidMACFinUsernameHeading,
	},
	{
		Name: "wrong MACFinIn Password Heading",
		PasswordManagerIn: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
		},
		PasswordManagerOut: []PasswordManagerRow{
			{
				"chris", "foo", "", -20 * Day,
			},
		},
		MACFinIn: []MACFinRow{
			{"chris", "foo"},
		},
		SheetInProblem: SheetProblemInvalidMACFinPasswordHeading,
	},
	{
		Name:               "empty MACFinIn; no headers",
		PasswordManagerIn:  []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
		SheetInProblem:     SheetProblemMACFinEmpty,
	},
	{
		Name:               "empty PasswordManagerIn; no headers",
		PasswordManagerIn:  []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
		SheetInProblem:     SheetProblemPasswordManagerEmpty,
	},
	{
		Name:               "PasswordManagerIn: too many cols",
		PasswordManagerIn:  []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
		SheetInProblem:     SheetProblemPasswordManagerTooManyHeadings,
	},
	{
		Name:               "PasswordManagerIn: too few cols",
		PasswordManagerIn:  []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
		SheetInProblem:     SheetProblemPasswordManagerTooFewHeadings,
	},
	{
		Name:               "PasswordManagerIn: wrong heading order",
		PasswordManagerIn:  []PasswordManagerRow{},
		PasswordManagerOut: []PasswordManagerRow{},
		MACFinIn:           []MACFinRow{},
		SheetInProblem:     SheetProblemPasswordManagerInvalidHeadingOrder,
	},
}

type FakeS3Client struct {
	Bucket, Key                    string
	LocalPath                      string
	AutomatedSheetName             string
	AutomatedSheetColNameToIndex   map[Column]int
	SheetName                      string
	UsernameHeader, PasswordHeader string
	RowOffset                      int
}

func (fc *FakeS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if aws.StringValue(params.Bucket) != fc.Bucket {
		return nil, fmt.Errorf("expected bucket %s; got %s", aws.StringValue(params.Bucket), fc.Bucket)
	}
	if aws.StringValue(params.Key) != fc.Key {
		return nil, fmt.Errorf("expected key %s; got %s", aws.StringValue(params.Key), fc.Key)
	}

	f, err := os.Open(fc.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("Error downloading s3 object: %s", err)
	}
	return &s3.GetObjectOutput{
		Body: f,
	}, nil
}

func (fc *FakeS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if aws.StringValue(params.Bucket) != fc.Bucket {
		return nil, fmt.Errorf("expected bucket %s; got %s", aws.StringValue(params.Bucket), fc.Bucket)
	}
	if aws.StringValue(params.Key) != fc.Key {
		return nil, fmt.Errorf("expected key %s; got %s", aws.StringValue(params.Key), fc.Key)
	}

	contents, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}

	// verify every password rotation has been uploaded except for, at most, one
	// compare uploaded file (fc.LocalPath) and current file (params.Body)
	err = verifyNewPasswordIsUploaded(fc, contents)
	if err != nil {
		return nil, fmt.Errorf("Error while checking for skipped file uploads: %s", err)
	}

	f, err := os.Create(fc.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("Error uploading file: %s", err)
	}
	defer f.Close()
	n, err := io.Copy(f, bytes.NewReader(contents))
	if err != nil {
		return nil, fmt.Errorf("Error uploading file %s: %s", fc.LocalPath, err)
	}

	log.Printf("opened %s (%d bytes) for upload to s3://%s/%s", fc.LocalPath, n, fc.Bucket, fc.Key)

	return nil, nil
}

type ColumnArrangement struct {
	Name    string
	Columns map[Column]int
}

var columnArrangements = []ColumnArrangement{
	{
		Name: "Normal",
		Columns: map[Column]int{
			ColUser: 0, ColPassword: 1, ColPrevious: 2, ColTimestamp: 3,
		},
	},
	{
		Name: "Reverse",
		Columns: map[Column]int{
			ColUser: 3, ColPassword: 2, ColPrevious: 1, ColTimestamp: 0,
		},
	},
	{
		Name: "User at end",
		Columns: map[Column]int{
			ColUser: 3, ColPassword: 0, ColPrevious: 1, ColTimestamp: 2,
		},
	},
}

var headings = map[Column]string{
	ColUser:      ColUserHeading,
	ColPassword:  ColPasswordHeading,
	ColPrevious:  ColPreviousHeading,
	ColTimestamp: ColTimestampHeading,
}

func TestRotate(t *testing.T) {
	for _, tc := range testCases {
		for _, arr := range columnArrangements {
			name := fmt.Sprintf("%s - Columns %s", tc.Name, arr.Name)
			cols := arr.Columns
			t.Run(name, func(t *testing.T) {
				dir, err := os.MkdirTemp(os.TempDir(), "macfin")
				if err != nil {
					t.Fatalf("Error making temp dir: %s", err)
				}
				log.Printf("Created %s", dir)
				defer os.RemoveAll(dir)
				filename := path.Join(dir, localS3Filename)
				f := excelize.NewFile()

				f.SetSheetName("Sheet1", sheetNameMACFin)
				f.NewSheet(sheetNamePasswordManager)

				var expectedSheetError error
				usernameHeading := headingMACFinUsername
				passwordHeading := headingMACFinPassword

				if tc.SheetInProblem == SheetProblemInvalidMACFinUsernameHeading {
					usernameHeading = "BadUsernameHeading"
					expectedSheetError = fmt.Errorf("sheet %s in file s3://%s/%s does not contain header %s in top row", sheetNameMACFin, inputBucket, inputKey, headingMACFinUsername)
				} else if tc.SheetInProblem == SheetProblemInvalidMACFinPasswordHeading {
					passwordHeading = "BadPasswordHeading"
					expectedSheetError = fmt.Errorf("sheet %s in file s3://%s/%s does not contain header %s in top row", sheetNameMACFin, inputBucket, inputKey, headingMACFinPassword)
				}

				if tc.SheetInProblem == SheetProblemMACFinEmpty {
					expectedSheetError = fmt.Errorf("sheet %s in file s3://%s/%s is empty; sheet must include header row", sheetNameMACFin, inputBucket, inputKey)
				} else {
					err = f.SetSheetRow(sheetNameMACFin, "A1", &[]string{
						"Module", "User_T", "Region", "State", usernameHeading, passwordHeading,
					})
					if err != nil {
						panic(err)
					}
				}

				for idx, row := range tc.MACFinIn {
					err := f.SetSheetRow(sheetNameMACFin, fmt.Sprintf("A%d", 2+idx), &[]string{
						"a", "b", "c", "d", row.Username, row.Password,
					})
					if err != nil {
						panic(err)
					}
				}

				h := make([]string, 5)
				h[cols[ColUser]] = headings[ColUser]
				h[cols[ColPassword]] = headings[ColPassword]
				h[cols[ColPrevious]] = headings[ColPrevious]
				h[cols[ColTimestamp]] = headings[ColTimestamp]

				if tc.SheetInProblem == SheetProblemPasswordManagerTooManyHeadings {
					h[4] = "Extra Heading"
					expectedSheetError = fmt.Errorf("Expected sheet %s to have %d cols; it has %d cols", sheetNamePasswordManager, len(cols), len(h))
				} else if tc.SheetInProblem == SheetProblemPasswordManagerInvalidHeadingOrder {
					h[cols[ColUser]] = headings[ColPassword]
					expectedSheetError = fmt.Errorf("Expected %s in col %d; got %s", headings[ColUser], cols[ColUser], h[cols[ColUser]])
				} else if tc.SheetInProblem == SheetProblemPasswordManagerTooFewHeadings {
					h = h[:2]
					expectedSheetError = fmt.Errorf("Expected sheet %s to have %d cols; it has %d cols", sheetNamePasswordManager, len(cols), len(h))
				}

				if tc.SheetInProblem == SheetProblemPasswordManagerEmpty {
					expectedSheetError = fmt.Errorf("sheet %s in file s3://%s/%s is empty; sheet must include header row", sheetNamePasswordManager, inputBucket, inputKey)
				} else {
					err = f.SetSheetRow(sheetNamePasswordManager, "A1", &h)
					if err != nil {
						panic(err)
					}
				}

				for idx, row := range tc.PasswordManagerIn {
					data := make([]string, 4)
					data[cols[ColUser]] = row.Username
					data[cols[ColPassword]] = row.Password
					data[cols[ColPrevious]] = row.Previous
					data[cols[ColTimestamp]] = format(row.Timestamp)
					err := f.SetSheetRow(sheetNamePasswordManager, fmt.Sprintf("A%d", 2+idx), &data)
					if err != nil {
						panic(err)
					}
				}

				f.SaveAs(filename)

				handler := &AuthServer{
					UserToPassword: make(map[string]string),
					Errors:         tc.ServerErrors,
				}
				for _, row := range tc.PasswordManagerIn {
					handler.UserToPassword[row.Username] = row.Password
				}
				if tc.UntrackedPasswords != nil {
					for username, password := range tc.UntrackedPasswords {
						handler.UserToPassword[strings.ToLower(username)] = password
					}
				}
				server := &http.Server{
					Addr:    ":3398",
					Handler: handler,
				}
				go func() {
					log.Printf("Server stopped: %s", server.ListenAndServe())
				}()

				input := &Input{
					UsernameHeader:                 headingMACFinUsername,
					PasswordHeader:                 headingMACFinPassword,
					Bucket:                         inputBucket,
					Key:                            inputKey,
					AutomatedSheetPassword:         "asfas",
					AutomatedSheetColNameToIndex:   cols,
					AutomatedSheetColNameToHeading: headings,
					RowOffset:                      1,
					SheetGroups: map[Environment]SheetGroup{
						dev: {
							AutomatedSheetName: "PasswordManager",
							SheetName:          sheetNameMACFin,
						},
					},
				}

				envToPortal := map[Environment]*Portal{
					dev: {
						Hostname:    portalServer,
						IDMHostname: idmServer,
						Scheme:      "http://",
					},
				}

				fc := &FakeS3Client{
					Bucket:                       input.Bucket,
					Key:                          input.Key,
					LocalPath:                    filename,
					AutomatedSheetName:           input.SheetGroups[dev].AutomatedSheetName,
					AutomatedSheetColNameToIndex: input.AutomatedSheetColNameToIndex,
					RowOffset:                    input.RowOffset,
					SheetName:                    input.SheetGroups[dev].SheetName,
					UsernameHeader:               input.UsernameHeader,
					PasswordHeader:               input.PasswordHeader,
				}
				err = rotate(input, envToPortal, fc)
				server.Shutdown(context.Background())

				if err != nil {
					if tc.SheetInProblem != NoSheetProblem {
						// check for input error
						if err.Error() == expectedSheetError.Error() {
							t.Logf("Error in input file: %s", err)
							return
						} else {
							t.Fatalf("Expected input sheet error %s, got %s", expectedSheetError, err)
						}
					} else {
						t.Fatalf("Error running rotate(): %s", err)
					}
				} else {
					if tc.SheetInProblem != NoSheetProblem {
						// expected input error
						t.Fatalf("Expected input sheet error %s, got %s", expectedSheetError, err)
					}
				}

				f, err = excelize.OpenFile(filename)
				if err != nil {
					t.Fatalf("Error reopening spreadsheet: %s", err)
				}

				pmRows, err := f.GetRows(sheetNamePasswordManager)
				if err != nil {
					t.Fatalf("Error getting Password Manager rows: %s", err)
				}
				if len(pmRows) < len(tc.PasswordManagerOut) {
					log.Fatalf("%s: Expected %d rows but got %d",
						sheetNamePasswordManager, len(tc.PasswordManagerOut), len(pmRows))
				}
				userToPassword := map[string]string{}
				for rowIdx, expected := range tc.PasswordManagerOut {
					gotRow := pmRows[rowIdx+1]
					gotUsername := gotRow[input.AutomatedSheetColNameToIndex[ColUser]]
					gotPassword := gotRow[input.AutomatedSheetColNameToIndex[ColPassword]]
					gotPrevious := gotRow[input.AutomatedSheetColNameToIndex[ColPrevious]]
					gotTS := gotRow[input.AutomatedSheetColNameToIndex[ColTimestamp]]
					if expected.Username != gotUsername {
						t.Fatalf("%s Row %d: expected Username=%s but got Username=%s",
							sheetNamePasswordManager, rowIdx+1, expected.Username, gotUsername)
					}
					// If the password has changed, we can't predict what it
					// will be, so just check that it matches what was sent to
					// the server.
					newPassword, ok := handler.UserToNewPassword[expected.Username]
					if ok {
						if expected.Password != newPasswordMarker {
							t.Fatalf("%s's password was changed unexpectedly", expected.Username)
						} else if newPassword != gotPassword {
							t.Fatalf("%s Row %d: new password recorded as %q but sent to server as %q",
								sheetNamePasswordManager, rowIdx+1, gotPassword, newPassword)
						}
						delete(handler.UserToNewPassword, expected.Username)
					} else if expected.Password == newPasswordMarker {
						t.Fatalf("Expected a password change for %s but the server did not get one", expected.Username)
					}
					if expected.Previous != gotPrevious {
						t.Fatalf("%s Row %d: expected Previous=%s but got Previous=%s",
							sheetNamePasswordManager, rowIdx+1, expected.Previous, gotPrevious)
					}
					expectedTimestamp := now.Add(expected.Timestamp)
					gotTimestamp, err := time.Parse(time.UnixDate, gotTS)
					if err != nil {
						t.Fatalf("%s Row %d: error parsing date %q: %s", sheetNamePasswordManager, rowIdx+1, gotTimestamp, err)
					}
					diff := expectedTimestamp.Sub(gotTimestamp)
					maxDiff := time.Hour
					if diff < -maxDiff || diff > maxDiff {
						t.Fatalf("%s Row %d: expected Timestamp~=%s but got Timestamp=%s",
							sheetNamePasswordManager, rowIdx+1, expectedTimestamp, gotTS)
					}
					userToPassword[gotUsername] = gotPassword
				}
				if len(handler.UserToNewPassword) != 0 {
					t.Fatalf("Passwords were updated for users not in the manager: %v", handler.UserToNewPassword)
				}

				macFinUsers := map[string]struct{}{}
				for _, row := range tc.MACFinIn {
					if row.Username != "" {
						macFinUsers[strings.ToLower(row.Username)] = struct{}{}
					}
				}
				macFinRows, err := f.GetRows(sheetNameMACFin)
				if err != nil {
					t.Fatalf("Error getting MACFin rows: %s", err)
				}

				for idx, row := range macFinRows[1:] {
					if len(row) < 6 {
						continue // short, invalid row
					}
					username := row[4]
					password := row[5]
					expectedPassword, ok := userToPassword[strings.ToLower(username)]
					if !ok {
						t.Fatalf("%s row %d: user %q is not in password manager",
							sheetNameMACFin, idx+1, username)
					} else if expectedPassword != password {
						t.Fatalf("%s row %d: Expected password %q but got %q",
							sheetNameMACFin, idx+1, expectedPassword, password)
					}
					delete(macFinUsers, username)
				}
				if len(macFinUsers) != 0 {
					t.Fatalf("Users are now missing from the MACFin sheet: %v", macFinUsers)
				}
				// Make sure all login sessions were terminated.
				for _, sess := range handler.PortalSessions {
					if tc.ServerErrors[sess.username] != "" {
						// When there is an error, we don't log out.
						continue
					}
					if !sess.loggedOut {
						t.Fatalf("Session %#v is still logged in", sess)
					}
				}
				for _, sess := range handler.IDMSessions {
					if tc.ServerErrors[sess.username] != "" {
						// When there is an error, we don't log out.
						continue
					}
					if !sess.loggedOut {
						t.Fatalf("Session %#v is still logged in", sess)
					}
				}
			})
		}
	}
}
