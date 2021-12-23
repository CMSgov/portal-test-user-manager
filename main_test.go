package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

var now = time.Now()

func format(d time.Duration) string {
	return now.Add(d).Format(time.UnixDate)
}

const newPasswordMarker = "newPassword"

const (
	sheetNameMACFin        = "MACFin"
	headingMACFinUsername  = "User"
	headinggMACFinPassword = "Password"

	sheetNamePasswordManager        = "PasswordManager"
	headingPasswordManagerUser      = "User"
	headingPasswordManagerPassword  = "Password"
	headingPasswordManagerPrevious  = "Previous"
	headingPasswordManagerTimestamp = "Timestamp"
)

func cn(col, row int) string {
	name, err := excelize.CoordinatesToCellName(col, row, false)
	if err != nil {
		panic(err)
	}
	return name
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

type TestCase struct {
	Name               string
	PasswordManagerIn  []PasswordManagerRow
	PasswordManagerOut []PasswordManagerRow
	MACFinIn           []MACFinRow
	UntrackedPasswords map[string]string // user -> password
	ServerErrors       map[string]string // user -> path
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
				"leslie", newPasswordMarker, "bar", 0,
			},
			{
				"james", "baz", "", -20 * Day,
			},
			{
				"chuck", newPasswordMarker, "y", 0,
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
			{"leslie", "bar"},
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
			{"james", "baz"},
			{"james", "baz"}, // duplicate
			{"chris", "foo"},
			{"leslie", "bar"},
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
				"james", newPasswordMarker, "baz", 0,
			},
			{
				"chris", newPasswordMarker, "foo", 0,
			},
			{
				"leslie", newPasswordMarker, "bar", 0,
			},
		},
		MACFinIn: []MACFinRow{
			{"james", "baz"},
			{"chris", "foo"},
			{"leslie", "bar"},
		},
		UntrackedPasswords: map[string]string{
			"leslie": "bar",
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
			{"chuck", "chuckie"},
		},
		UntrackedPasswords: map[string]string{
			"chuck": "chuckie",
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
}

func TestRotate(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			dir, err := os.MkdirTemp(os.TempDir(), "test-user-manager")
			if err != nil {
				t.Fatalf("Error making temp dir: %s", err)
			}
			log.Printf("Created %s", dir)
			defer os.RemoveAll(dir)
			filename := path.Join(dir, "in.xlsx")
			f := excelize.NewFile()

			f.SetSheetName("Sheet1", sheetNameMACFin)
			f.NewSheet(sheetNamePasswordManager)

			err = f.SetSheetRow(sheetNameMACFin, "A1", &[]string{
				"Module", "User_T", "Region", "State", headingMACFinUsername, headinggMACFinPassword,
			})
			if err != nil {
				panic(err)
			}
			for idx, row := range tc.MACFinIn {
				err := f.SetSheetRow(sheetNameMACFin, "A"+string(rune('2'+idx)), &[]string{
					"a", "b", "c", "d", row.Username, row.Password,
				})
				if err != nil {
					panic(err)
				}
			}

			err = f.SetSheetRow(sheetNamePasswordManager, "A1", &[]string{
				headingPasswordManagerUser,
				headingPasswordManagerPassword,
				headingPasswordManagerPrevious,
				headingPasswordManagerTimestamp,
			})
			if err != nil {
				panic(err)
			}
			for idx, row := range tc.PasswordManagerIn {
				err := f.SetSheetRow(sheetNamePasswordManager, "A"+string(rune('2'+idx)), &[]string{
					row.Username, row.Password, row.Previous, format(row.Timestamp),
				})
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
					handler.UserToPassword[username] = password
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
				SheetName:              sheetNameMACFin,
				UsernameHeader:         headingMACFinUsername,
				PasswordHeader:         headinggMACFinPassword,
				Filename:               filename,
				AutomatedSheetPassword: "asfas",
				AutomatedSheetName:     "PasswordManager",
				AutomatedSheetColNameToIndex: map[Column]int{
					ColUser: 0, ColPassword: 1, ColPrevious: 2, ColTimestamp: 3},
				RowOffset: 1,
			}

			portal := &Portal{
				Hostname:    portalServer,
				IDMHostname: idmServer,
				Scheme:      "http://",
			}

			err = rotate(input, portal)
			if err != nil {
				t.Fatalf("Error running rotate(): %s", err)
			}

			server.Shutdown(context.Background())

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
				got := pmRows[rowIdx+1]
				if expected.Username != got[0] {
					t.Fatalf("%s Row %d: expected Username=%s but got Username=%s",
						sheetNamePasswordManager, rowIdx+1, expected.Username, got[0])
				}
				// If the password has changed, we can't predict what it
				// will be, so just check that it matches what was sent to
				// the server.
				newPassword, ok := handler.UserToNewPassword[expected.Username]
				if ok {
					if expected.Password != newPasswordMarker {
						t.Fatalf("%s's password was changed unexpectedly", expected.Username)
					} else if newPassword != got[1] {
						t.Fatalf("%s Row %d: new password recorded as %q but sent to server as %q",
							sheetNamePasswordManager, rowIdx+1, got[1], newPassword)
					}
					delete(handler.UserToNewPassword, expected.Username)
				} else if expected.Password == newPasswordMarker {
					t.Fatalf("Expected a password change for %s but the server did not get one", expected.Username)
				}
				if expected.Previous != got[2] {
					t.Fatalf("%s Row %d: expected Previous=%s but got Previous=%s",
						sheetNamePasswordManager, rowIdx+1, expected.Previous, got[2])
				}
				expectedTimestamp := now.Add(expected.Timestamp)
				gotTimestamp, err := time.Parse(time.UnixDate, got[3])
				if err != nil {
					t.Fatalf("%s Row %d: error parsing date %q: %s", sheetNamePasswordManager, rowIdx+1, got[3], err)
				}
				diff := expectedTimestamp.Sub(gotTimestamp)
				maxDiff := time.Hour
				if diff < -maxDiff || diff > maxDiff {
					t.Fatalf("%s Row %d: expected Timestamp~=%s but got Timestamp=%s",
						sheetNamePasswordManager, rowIdx+1, expectedTimestamp, gotTimestamp)
				}
				userToPassword[got[0]] = got[1]
			}
			if len(handler.UserToNewPassword) != 0 {
				t.Fatalf("Passwords were updated for users not in the manager: %v", handler.UserToNewPassword)
			}

			macFinUsers := map[string]struct{}{}
			for _, row := range tc.MACFinIn {
				if row.Username != "" {
					macFinUsers[row.Username] = struct{}{}
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
				expectedPassword, ok := userToPassword[username]
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
