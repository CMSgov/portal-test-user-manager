package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
)

const (
	loginClearPath        = "/portal/selfservice/users/loginClear/"
	loginSubmitPath       = "/portal/login"
	loginOauth2Path       = "/login/sessionCookieRedirect"
	oauth2RedirectUrlPath = "/myportal/"
	changePasswordPath    = "/myportal/viewprofile/myprofile/credential"
	logoutPath            = "/myportal/logout"
	sessionToken          = "sessionToken"
)

type loginDetails struct {
	Username string          `json:"username"`
	Password string          `json:"password"`
	Options  map[string]bool `json:"options"`
}

type changePassword struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

type userData struct {
	SessionToken string `json:"sessionToken"`
}

func getCookie(c *http.Client, urlstr, cookieName string) (*http.Cookie, error) {
	urlObj, err := url.Parse(urlstr)
	if err != nil {
		panic(err)
	}
	cookies := c.Jar.Cookies(urlObj)
	for _, c := range cookies {
		if c.Name == cookieName {
			return c, nil
		}
	}
	return nil, fmt.Errorf("failed to find %s in cookie jar", cookieName)
}

func sendRequest(client *http.Client, method, urlstr string, customHeaders map[string][]string, body []byte, userData interface{}) error {
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequest(http.MethodGet, urlstr, nil)
	} else if method == http.MethodPost {
		req, err = http.NewRequest(http.MethodPost, urlstr, bytes.NewReader(body))
	} else {
		return errors.New("unsupported method type")
	}

	if err != nil {
		return fmt.Errorf("Error creating request: %s", err)
	}

	if len(body) > 0 {
		req.Header.Add("Content-Type", contentType)
	}

	// Add common headers
	req = addHeadersToRequest(req, commonHeaders)

	// Add custom headers
	req = addHeadersToRequest(req, customHeaders)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("failed to read response body: %s", err)
		}
		return fmt.Errorf("got HTTP status code %d with body: \n%s", resp.StatusCode, string(b))
	}

	if userData != nil {
		err = json.NewDecoder(resp.Body).Decode(userData)
		if err != nil {
			return fmt.Errorf("Error decoding response body: %s", err)
		}
	}

	return nil
}

func loginStep(client *http.Client, portal *Portal, username, password string) error {
	hostname := portal.Hostname
	idmHostname := portal.IDMHostname

	// GET loginClearPath adds 4 cookies to the jar:
	// portal.cms.gov: dc, DC, akavpau_default, IDMSession
	// Note: portaldev.cms.gov does not return the DC cookie
	headers := map[string][]string{
		"sec-fetch-site": {"same-origin"},
		"sec-fetch-mode": {"cors"},
		"sec-fetch-dest": {"empty"},
		"pragma":         {"no-cache"},
		"referer":        {portal.Scheme + hostname},
	}

	err := sendRequest(client, http.MethodGet, portal.Scheme+hostname+loginClearPath, headers, nil, nil)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}

	// POST loginSubmitPath returns a sessionToken in the response body
	// sessionToken is a query parameter in the GET to oauth2RedirectUrlPath
	// Returns no new cookies.
	creds := loginDetails{
		Username: username,
		Password: password,
		Options:  map[string]bool{"warnBeforePasswordExpired": true},
	}

	body, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("Error marshalling creds: %s", err)
	}

	headers = map[string][]string{
		"sec-fetch-site": {"same-origin"},
		"sec-fetch-mode": {"cors"},
		"sec-fetch-dest": {"empty"},
		"pragma":         {"no-cache"},
		"referer":        {portal.Scheme + hostname + "/portal/"},
		"origin":         {hostname},
	}

	userData := &userData{}
	err = sendRequest(client, http.MethodPost, portal.Scheme+hostname+loginSubmitPath, headers, body, userData)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}

	if userData.SessionToken == "" {
		return fmt.Errorf("Error no session token: %s", errors.New("missing sessionToken in response body; user might be locked out of portal"))
	}

	// Start the oauth2 process between client and server
	// GET to oauth2RedirectUrlPath
	// Response returns 12 cookies: 4 existing cookies and 8 new ones
	// New cookies for portal.cms.gov: F5_ST, LastMRH_Session, MRHSession, PORTAL-XSRF-TOKEN
	// New cookies for idm.cms.gov: t, DT, JSESSIONID, sid
	token := userData.SessionToken
	params := url.Values{}
	params.Add("token", token)
	params.Add("redirectUrl", fmt.Sprintf("%s%s%s", portal.Scheme, hostname, oauth2RedirectUrlPath))
	urlObj, err := url.Parse(portal.Scheme + idmHostname + loginOauth2Path)
	if err != nil {
		return fmt.Errorf("Error logging in: %s", err)
	}
	urlObj.RawQuery = params.Encode()
	headers = map[string][]string{
		"upgrade-insecure-requests": {"1"},
		"sec-fetch-site":            {"same-site"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-user":            {"?1"},
		"referer":                   {portal.Scheme + hostname},
		"origin":                    {hostname},
	}
	err = sendRequest(client, http.MethodGet, urlObj.String(), headers, nil, nil)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}

	return nil
}

func changePasswordStep(client *http.Client, portal *Portal, oldPassword, newPassword string) error {
	hostname := portal.Hostname

	// POST to changePasswordPath
	// Request headers and cookie jar contain PORTAL-XSRF-TOKEN
	// Request body contains credentials
	// If request succeeds, then password is reset
	// No new cookies added.
	creds := changePassword{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	body, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("Error marshalling creds: %s", err)
	}

	portalXsrfTokenCookie, err := getCookie(client, portal.Scheme+hostname, "PORTAL-XSRF-TOKEN")
	if err != nil {
		return fmt.Errorf("Error getting cookie from jar: %s", err)
	}
	headers := map[string][]string{
		"sec-fetch-site":    {"same-origin"},
		"sec-fetch-mode":    {"cors"},
		"sec-fetch-dest":    {"empty"},
		"referer":           {portal.Scheme + hostname + "/myportal/view-profile"},
		"origin":            {portal.Scheme + hostname},
		"xhr_request":       {"true"},
		"observe":           {"response"},
		"portal-xsrf-token": {portalXsrfTokenCookie.Value},
	}

	err = sendRequest(client, http.MethodPost, portal.Scheme+hostname+changePasswordPath, headers, body, nil)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}
	return nil
}

func changeUserPassword(client *http.Client, portal *Portal, username, oldPassword, newPassword string) error {
	err := loginStep(client, portal, username, oldPassword)
	if err != nil {
		return fmt.Errorf("Error logging in: %s", err)
	}

	err = changePasswordStep(client, portal, oldPassword, newPassword)
	if err != nil {
		return fmt.Errorf("Error changing password: %s", err)
	}

	err = logoutStep(client, portal)
	if err != nil {
		return fmt.Errorf("Error logging out: %s", err)
	}
	return nil
}

func logoutStep(client *http.Client, portal *Portal) (err error) {
	hostname := portal.Hostname
	err = sendRequest(client, http.MethodGet, portal.Scheme+hostname+logoutPath, nil, nil, nil)
	if err != nil {
		return fmt.Errorf("Error sending request: %s", err)
	}

	return nil
}
