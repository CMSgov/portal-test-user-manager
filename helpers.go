package main

import (
	"net/http"
	"net/url"

	"github.com/xuri/excelize/v2"
)

func getCookie(c *http.Client, urlstr, cookieName string) *http.Cookie {
	urlObj, _ := url.Parse(urlstr)
	cookies := c.Jar.Cookies(urlObj)
	for _, c := range cookies {
		if c.Name == cookieName {
			return c
		}
	}
	return nil
}

func validateFilenameLength(input *Input) error {
	filename := input.Filename
	runes := []rune(filename)
	if len(runes) > excelize.MaxFileNameLength {
		return excelize.ErrMaxFileNameLength
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
