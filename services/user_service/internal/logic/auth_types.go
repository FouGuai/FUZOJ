package logic

import "time"

type RegisterInput struct {
	Username string
	Password string
}

type LoginInput struct {
	Username   string
	Password   string
	IP         string
	DeviceInfo string
}

type RefreshInput struct {
	RefreshToken string
}

type LogoutInput struct {
	RefreshToken string
}

type UserInfo struct {
	ID       int64
	Username string
	Role     string
}

type AuthResult struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	User             UserInfo
}
