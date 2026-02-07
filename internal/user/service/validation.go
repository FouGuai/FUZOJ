package service

import (
	"regexp"

	pkgerrors "fuzoj/pkg/errors"
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return pkgerrors.New(pkgerrors.InvalidUsername)
	}
	return nil
}

// add more validation logical
func validatePassword(password string) error {
	if len(password) < 8 {
		return pkgerrors.New(pkgerrors.PasswordTooWeak)
	}
	if len(password) > 128 {
		return pkgerrors.New(pkgerrors.InvalidPassword)
	}
	return nil
}
