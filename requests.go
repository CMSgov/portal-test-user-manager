package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

const (
	scheme                = "https://"
	loginPagePath         = "/portal/"
	loginClearPath        = "/portal/selfservice/users/loginClear/"
	loginSubmitPath       = "/portal/login"
	loginOauth2Path       = "/login/sessionCookieRedirect?"
	oauth2RedirectUrlPath = "/myportal/"
	userDataPath          = "/api/v1/sessions/me/lifecycle/refresh"
	changePasswordPath    = "/myportal/viewprofile/myprofile/credential"
	logoutPath            = "/myportal/logout"
	userId                = "userId"
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

type UserData struct {
	UserId       string `json:"userId"`
	SessionToken string `json:"sessionToken"`
}

func sendRequest(client *http.Client, method, urlstr string, customHeaders map[string][]string, body []byte) (*UserData, error) {
	var req *http.Request
	var err error
	userData := &UserData{}
	if method == http.MethodGet {
		req, err = http.NewRequest(http.MethodGet, urlstr, nil)
	} else if method == http.MethodPost {
		req, err = http.NewRequest(http.MethodPost, urlstr, bytes.NewReader(body))
	} else {
		return nil, errors.New("unsupported method type")
	}

	if err != nil {
		return nil, err
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
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(b))
	}

	if method == http.MethodPost && (strings.HasSuffix(urlstr, loginSubmitPath) ||
		strings.HasSuffix(urlstr, userDataPath)) {
		err = json.NewDecoder(resp.Body).Decode(userData)
		if err != nil {
			return nil, err
		}
	}

	return userData, nil
}

func loginStep(client *http.Client, config *Portal, username, password string) error {
	// GET login page at https://portal.cms.gov/portal/
	hostname := config.Hostname
	idmHostname := config.IdmHostname

	headers := map[string][]string{
		"Host":                      {hostname},
		"upgrade-insecure-requests": {"1"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-user":            {"?1"},
		"sec-fetch-dest":            {"document"},
	}

	_, err := sendRequest(client, http.MethodGet, scheme+hostname+loginPagePath, headers, nil)
	if err != nil {
		return err
	}

	// This request GETs the IDMSession Cookie.
	headers = map[string][]string{
		"Host":           {hostname},
		"sec-fetch-site": {"same-origin"},
		"sec-fetch-mode": {"cors"},
		"sec-fetch-dest": {"empty"},
		"pragma":         {"no-cache"},
		"referer":        {scheme + hostname},
	}

	_, err = sendRequest(client, http.MethodGet, scheme+hostname+loginClearPath, headers, nil)
	if err != nil {
		return err
	}

	// POST  https://portal.cms.gov/portal/login to get sessionToken used for oauth2 token
	creds := loginDetails{
		Username: username,
		Password: password,
		Options:  map[string]bool{"warnBeforePasswordExpired": true},
	}

	body, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	headers = map[string][]string{
		"Host":           {hostname},
		"sec-fetch-site": {"same-origin"},
		"sec-fetch-mode": {"cors"},
		"sec-fetch-dest": {"empty"},
		"pragma":         {"no-cache"},
		"referer":        {scheme + hostname + "/portal/"},
		"origin":         {hostname},
	}

	userData, err := sendRequest(client, http.MethodPost, scheme+hostname+loginSubmitPath, headers, body)
	if err != nil {
		return err
	}

	if userData.SessionToken == "" {
		return errors.New("missing sessionToken in response body; user might be locked out of portal")
	}

	// Start the oauth2 process
	// GET https://idm.cms.gov/login/sessionCookieRedirect?token=&redirectUrl=https://portal.cms.gov/myportal/
	// get the sessionToken from the response body of the POST /portal/login request and use as oauth2 token
	token := userData.SessionToken
	params := fmt.Sprintf("token=%s&redirectUrl=%s%s%s", token, scheme, hostname, oauth2RedirectUrlPath)

	headers = map[string][]string{
		"Host":                      {idmHostname},
		"upgrade-insecure-requests": {"1"},
		"sec-fetch-site":            {"same-site"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-user":            {"?1"},
		"referer":                   {scheme + hostname},
		"origin":                    {hostname},
	}
	_, err = sendRequest(client, http.MethodGet, scheme+idmHostname+loginOauth2Path+params, headers, nil)
	if err != nil {
		return err
	}

	return nil
}

func changePasswordStep(client *http.Client, config *Portal, oldPassword, newPassword string) error {
	// Get userID from response body of:
	// GET https://idm.cms.gov/api/v1/sessions/me/lifecycle/refresh
	hostname := config.Hostname
	idmHostname := config.IdmHostname

	headers := map[string][]string{
		"Host":                       {idmHostname},
		"x-okta-user-agent-extended": {"okta-auth-js-1.8.0"}, // brittle
		"x-requested-with":           {"XMLHttpRequest"},
		"sec-fetch-site":             {"same-site"},
		"sec-fetch-mode":             {"cors"},
		"sec-fetch-dest":             {"empty"},
		"referer":                    {scheme + hostname + "/myportal/view-profile"},
		"origin":                     {scheme + hostname},
	}

	userData, err := sendRequest(client, http.MethodPost, scheme+idmHostname+userDataPath, headers, nil)
	if err != nil {
		return err
	}

	if userData.UserId == "" {
		return errors.New("missing userId in response body")
	}

	// Change password: https://portal.cms.gov/myportal/viewprofile/myprofile/credential
	creds := changePassword{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	body, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	portalXsrfTokenCookie := getCookie(client, scheme+hostname, "PORTAL-XSRF-TOKEN")
	headers = map[string][]string{
		"Host":              {hostname},
		"sec-fetch-site":    {"same-origin"},
		"sec-fetch-mode":    {"cors"},
		"sec-fetch-dest":    {"empty"},
		"referer":           {scheme + hostname + "/myportal/view-profile"},
		"origin":            {scheme + hostname},
		"xhr_request":       {"true"},
		"observe":           {"response"},
		"portal-xsrf-token": {portalXsrfTokenCookie.Value},
		"userid":            {userData.UserId},
	}

	_, err = sendRequest(client, http.MethodPost, scheme+hostname+changePasswordPath, headers, body)
	if err != nil {
		return err
	}
	return nil
}

func changeUserPassword(client *http.Client, config *Portal, username, oldPassword, newPassword string) error {
	err := loginStep(client, config, username, oldPassword)
	if err != nil {
		return err
	}

	err = changePasswordStep(client, config, oldPassword, newPassword)
	if err != nil {
		return err
	}

	err = logoutStep(client, config)
	if err != nil {
		return err
	}
	return nil
}

func logoutStep(client *http.Client, config *Portal) error {
	hostname := config.Hostname
	_, err := sendRequest(client, http.MethodGet, scheme+hostname+logoutPath, nil, nil)
	// Delete all cookies.
	defer func(err error) error {
		client.Jar, _ = cookiejar.New(nil)
		return err
	}(err)

	if err != nil {
		return err
	}
	return nil
}
