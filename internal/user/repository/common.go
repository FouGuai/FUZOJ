package repository

import "errors"

const (
	userBannedKey     = "user:banned"
	tokenBlacklistKey = "token:blacklist"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrTokenNotFound  = errors.New("token not found")
	ErrDuplicate      = errors.New("record already exists")
	ErrUsernameExists = errors.New("username already exists")
	ErrEmailExists    = errors.New("email already exists")
)
