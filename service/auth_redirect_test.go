package service

import "testing"

func TestSafeRedirectPath(t *testing.T) {
	cases := map[string]string{
		"/":                   "/",
		"/canvas/abc":         "/canvas/abc",
		"/login?redirect=/x":  "/login?redirect=/x",
		"":                    "/",
		"//evil.com":          "/",
		"/\\evil.com":         "/",
		"https://evil.com":    "/",
		"http://evil.com":     "/",
		"javascript:alert(1)": "/",
		"evil.com":            "/",
		"/\t/evil.com":        "/", // browsers strip the tab → //evil.com
		"/normal\tpath":       "/normalpath",
	}
	for in, want := range cases {
		if got := safeRedirectPath(in); got != want {
			t.Errorf("safeRedirectPath(%q) = %q, want %q", in, got, want)
		}
	}
}
