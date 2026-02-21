package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"fuzoj/internal/common/db"
	"fuzoj/user_service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type UserStatus string

const (
	UserStatusActive        UserStatus = "active"
	UserStatusBanned        UserStatus = "banned"
	UserStatusPendingVerify UserStatus = "pending_verify"
)

type UserRole string

const (
	UserRoleGuest          UserRole = "guest"
	UserRoleUser           UserRole = "user"
	UserRoleProblemSetter  UserRole = "problem_setter"
	UserRoleContestManager UserRole = "contest_manager"
	UserRoleAdmin          UserRole = "admin"
	UserRoleSuperAdmin     UserRole = "super_admin"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	Phone        *string
	PasswordHash string
	Role         UserRole
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserRepository interface {
	Create(ctx context.Context, user *User) (int64, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdatePassword(ctx context.Context, userID int64, newHash string) error
	UpdateStatus(ctx context.Context, userID int64, status UserStatus) error
	UpdateRole(ctx context.Context, userID int64, role UserRole) error
	UpdateEmail(ctx context.Context, userID int64, email string) error
	WithSession(session sqlx.Session) UserRepository
}

type MySQLUserRepository struct {
	usersModel model.UsersModel
}

func NewUserRepository(usersModel model.UsersModel) UserRepository {
	return &MySQLUserRepository{
		usersModel: usersModel,
	}
}

func (r *MySQLUserRepository) WithSession(session sqlx.Session) UserRepository {
	if session == nil {
		return r
	}
	return &MySQLUserRepository{
		usersModel: r.usersModel.WithSession(session),
	}
}

func (r *MySQLUserRepository) Create(ctx context.Context, user *User) (int64, error) {
	if user == nil {
		return 0, errors.New("user is nil")
	}

	role := user.Role
	if role == "" {
		role = UserRoleUser
	}
	status := user.Status
	if status == "" {
		status = UserStatusPendingVerify
	}

	phone := sql.NullString{}
	if user.Phone != nil {
		phone = sql.NullString{String: *user.Phone, Valid: true}
	}

	result, err := r.usersModel.Insert(ctx, &model.Users{
		Username:     user.Username,
		Email:        user.Email,
		Phone:        phone,
		PasswordHash: user.PasswordHash,
		Role:         string(role),
		Status:       string(status),
	})
	if err != nil {
		if key, ok := db.UniqueViolation(err); ok {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			switch {
			case strings.Contains(normalizedKey, "users_username_uq") || strings.Contains(normalizedKey, "users.username") || strings.Contains(normalizedKey, ".username") || strings.Contains(normalizedKey, "username"):
				return 0, ErrUsernameExists
			case strings.Contains(normalizedKey, "users_email_uq") || strings.Contains(normalizedKey, "users.email") || strings.Contains(normalizedKey, ".email") || strings.Contains(normalizedKey, "email"):
				return 0, ErrEmailExists
			default:
				return 0, ErrDuplicate
			}
		}
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (r *MySQLUserRepository) GetByID(ctx context.Context, id int64) (*User, error) {
	result, err := r.usersModel.FindOne(ctx, id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return fromUserModel(result), nil
}

func (r *MySQLUserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	result, err := r.usersModel.FindOneByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return fromUserModel(result), nil
}

func (r *MySQLUserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	result, err := r.usersModel.FindOneByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return fromUserModel(result), nil
}

func (r *MySQLUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	_, err := r.usersModel.FindOneByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MySQLUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	_, err := r.usersModel.FindOneByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MySQLUserRepository) UpdatePassword(ctx context.Context, userID int64, newHash string) error {
	existing, err := r.usersModel.FindOne(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	existing.PasswordHash = newHash
	return r.usersModel.Update(ctx, existing)
}

func (r *MySQLUserRepository) UpdateStatus(ctx context.Context, userID int64, status UserStatus) error {
	existing, err := r.usersModel.FindOne(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	existing.Status = string(status)
	return r.usersModel.Update(ctx, existing)
}

func (r *MySQLUserRepository) UpdateRole(ctx context.Context, userID int64, role UserRole) error {
	existing, err := r.usersModel.FindOne(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	existing.Role = string(role)
	return r.usersModel.Update(ctx, existing)
}

func (r *MySQLUserRepository) UpdateEmail(ctx context.Context, userID int64, email string) error {
	existing, err := r.usersModel.FindOne(ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	existing.Email = email
	if err := r.usersModel.Update(ctx, existing); err != nil {
		if key, ok := db.UniqueViolation(err); ok {
			normalizedKey := strings.ToLower(strings.TrimSpace(key))
			if strings.Contains(normalizedKey, "users_email_uq") || strings.Contains(normalizedKey, "users.email") || strings.Contains(normalizedKey, ".email") || strings.Contains(normalizedKey, "email") {
				return ErrEmailExists
			}
			return ErrDuplicate
		}
		return err
	}
	return nil
}

func fromUserModel(data *model.Users) *User {
	if data == nil {
		return nil
	}
	var phone *string
	if data.Phone.Valid {
		phone = &data.Phone.String
	}
	return &User{
		ID:           data.Id,
		Username:     data.Username,
		Email:        data.Email,
		Phone:        phone,
		PasswordHash: data.PasswordHash,
		Role:         UserRole(data.Role),
		Status:       UserStatus(data.Status),
		CreatedAt:    data.CreatedAt,
		UpdatedAt:    data.UpdatedAt,
	}
}
