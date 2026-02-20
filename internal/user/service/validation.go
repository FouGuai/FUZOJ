package service

import (
	"regexp"

	pkgerrors "fuzoj/pkg/errors"
)

// Username: 3-32 chars, start with a letter, allow letters, numbers, dot, underscore, hyphen.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]{2,31}$`)

// Password: 8-128 chars, must contain at least one letter and one number, printable ASCII only.
var passwordPattern = regexp.MustCompile(`^[\x21-\x7E]{8,128}$`)

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return pkgerrors.New(pkgerrors.InvalidUsername)
	}
	return nil
}

// add more validation logical
func validatePassword(password string) error {
	if !passwordPattern.MatchString(password) {
		if len(password) < 8 {
			return pkgerrors.New(pkgerrors.PasswordTooWeak)
		}
		return pkgerrors.New(pkgerrors.InvalidPassword)
	}
	if !hasLetterAndNumber(password) {
		return pkgerrors.New(pkgerrors.PasswordTooWeak)
	}
	return nil
}

func validateLoginPassword(password string) error {
	if len(password) < 8 {
		return pkgerrors.New(pkgerrors.PasswordTooWeak)
	}
	if len(password) > 128 {
		return pkgerrors.New(pkgerrors.InvalidPassword)
	}
	return nil
}

func hasLetterAndNumber(password string) bool {
	hasLetter := false
	hasNumber := false
	for i := 0; i < len(password); i++ {
		b := password[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
			hasLetter = true
		} else if b >= '0' && b <= '9' {
			hasNumber = true
		}
		if hasLetter && hasNumber {
			return true
		}
	}
	return false
}
