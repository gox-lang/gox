package gox

import (
	"context"
	"fmt"
	"net"
	"strings"
	"unicode"
)

type Email struct {
	address  string
	username string
	domain   string
	valid    bool
}

func NewEmail(address string) (Email, error) {
	address = strings.TrimSpace(address)
	at := strings.IndexByte(address, '@')
	if at <= 0 || at != strings.LastIndexByte(address, '@') || at == len(address)-1 {
		return Email{}, fmt.Errorf("invalid email address")
	}
	username := address[:at]
	domain := strings.ToLower(address[at+1:])
	if !validEmailUsername(username) || !validEmailDomain(domain) {
		return Email{}, fmt.Errorf("invalid email address")
	}
	return Email{address: username + "@" + domain, username: username, domain: domain, valid: true}, nil
}

func (e Email) Address() string  { return e.address }
func (e Email) Username() string { return e.username }
func (e Email) Domain() string   { return e.domain }
func (e Email) String() string   { return e.address }
func (e Email) IsZero() bool     { return !e.valid }

func (e Email) CheckMX(ctx context.Context) (bool, error) {
	if e.IsZero() {
		return false, fmt.Errorf("invalid email address")
	}
	resolver := net.DefaultResolver
	mx, err := resolver.LookupMX(ctx, e.domain)
	if err != nil {
		return false, err
	}
	return len(mx) > 0, nil
}

func validEmailUsername(value string) bool {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', '_', '%', '+', '-':
			continue
		default:
			return false
		}
	}
	return true
}

func validEmailDomain(value string) bool {
	if value == "" || !strings.Contains(value, ".") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}
