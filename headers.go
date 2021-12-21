package main

import (
	"fmt"
	"net/http"
)

const (
	contentType = "application/json"
	userAgent   = "Mozilla/5.0 (platform; rv:geckoversion) Gecko/geckotrail Firefox/firefoxversion"
)

var commonHeaders = map[string][]string{
	"Accept":          {"*/*"},
	"Accept-Encoding": {"gizp", "deflate", "br"},
	"cache-control":   {"no-cache", "no-store", "must-revalidate", "post-check=0", "pre-check=0"},
	"Connection":      {"keep-alive"},
	"dnt":             {"1"},
	"sec-ch-ua": {
		fmt.Sprintf("%q;v=%q", "Google Chrome", "95"),
		fmt.Sprintf("%q;v=%q", "Chromium", "95"),
		fmt.Sprintf("%q;v=%q;", ";Not A Brand", "99"),
	},
	"sec-ch-ua-mobile":   {"?0"},
	"sec-ch_ua_platform": {"Windows"},
	"User-Agent":         {userAgent},
}

func addHeadersToRequest(request *http.Request, hdrs map[string][]string) *http.Request {
	for k, vs := range hdrs {
		for _, v := range vs {
			request.Header.Add(k, v)
		}
	}

	return request
}
