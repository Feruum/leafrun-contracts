// Package httpauth provides explicit public cookie transport without an ambient jar.
package httpauth

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Cookie struct{ name, value, origin string }

func NewCookie(name, value, origin string, loopbackHTTP bool) (*Cookie, error) {
	if len(name) > 128 || name == "" || value == "" || len(value) > 4096 || (&http.Cookie{Name: name, Value: value}).Valid() != nil {
		return nil, errors.New("invalid explicit cookie")
	}
	for _, b := range []byte(value) {
		if b < 0x21 || b > 0x7e || strings.ContainsRune("\"\\;,", rune(b)) {
			return nil, errors.New("invalid explicit cookie")
		}
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.String() != origin || strings.ToLower(u.Host) != u.Host || len(origin) > 2048 || strings.ContainsAny(origin, " \t\r\n,\\") {
		return nil, errors.New("invalid explicit origin")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if !loopbackHTTP || u.Scheme != "http" || (u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) {
			return nil, errors.New("origin requires HTTPS")
		}
	}
	if port := u.Port(); port != "" {
		if _, err := net.LookupPort("tcp", port); err != nil {
			return nil, errors.New("invalid origin port")
		}
	}
	return &Cookie{name: name, value: value, origin: origin}, nil
}
func (c *Cookie) Apply(r *http.Request) error {
	if c == nil || r == nil || !strings.HasPrefix(r.URL.Path, "/v1/") {
		return errors.New("cookie transport requires public route")
	}
	if r.Header.Get("Authorization") != "" || len(r.Header.Values("Cookie")) != 0 {
		return errors.New("ambiguous explicit authentication")
	}
	r.Header.Set("Cookie", c.name+"="+c.value)
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
	default:
		r.Header.Set("Origin", c.origin)
	}
	return nil
}
