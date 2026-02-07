package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"fuzoj/internal/common/db"
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
	Create(ctx context.Context, tx db.Transaction, user *User) (int64, error)
	GetByID(ctx context.Context, tx db.Transaction, id int64) (*User, error)
	GetByUsername(ctx context.Context, tx db.Transaction, username string) (*User, error)
	GetByEmail(ctx context.Context, tx db.Transaction, email string) (*User, error)
	ExistsByUsername(ctx context.Context, tx db.Transaction, username string) (bool, error)
	ExistsByEmail(ctx context.Context, tx db.Transaction, email string) (bool, error)
	UpdatePassword(ctx context.Context, tx db.Transaction, userID int64, newHash string) error
	UpdateStatus(ctx context.Context, tx db.Transaction, userID int64, status UserStatus) error
}

type PostgresUserRepository struct {
	db db.Database
}

func NewUserRepository(database db.Database) UserRepository {
	return &PostgresUserRepository{db: database}
}

const userColumns = "id, username, email, phone, password_hash, role, status, created_at, updated_at"

func (r *PostgresUserRepository) Create(ctx context.Context, tx db.Transaction, user *User) (int64, error) {
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

	query := "INSERT INTO users (username, email, phone, password_hash, role, status) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, user.Username, user.Email, phone, user.PasswordHash, role, status)
	var id int64
	if err := row.Scan(&id); err != nil {
		if pqErr, ok := uniqueViolation(err); ok {
			switch pqErr.Constraint {
			case "users_username_uq":
				return 0, ErrUsernameExists
			case "users_email_uq":
				return 0, ErrEmailExists
			default:
				return 0, ErrDuplicate
			}
		}
		return 0, err
	}
	return id, nil
}

func (r *PostgresUserRepository) GetByID(ctx context.Context, tx db.Transaction, id int64) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE id = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, id)
	user, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *PostgresUserRepository) GetByUsername(ctx context.Context, tx db.Transaction, username string) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE username = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, username)
	user, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *PostgresUserRepository) GetByEmail(ctx context.Context, tx db.Transaction, email string) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE email = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, email)
	user, err := scanUser(row)
	if err != nil {
		if isNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *PostgresUserRepository) ExistsByUsername(ctx context.Context, tx db.Transaction, username string) (bool, error) {
	query := "SELECT 1 FROM users WHERE username = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, username)
	var one int
	if err := row.Scan(&one); err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *PostgresUserRepository) ExistsByEmail(ctx context.Context, tx db.Transaction, email string) (bool, error) {
	query := "SELECT 1 FROM users WHERE email = $1"
	row := getQuerier(r.db, tx).QueryRow(ctx, query, email)
	var one int
	if err := row.Scan(&one); err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *PostgresUserRepository) UpdatePassword(ctx context.Context, tx db.Transaction, userID int64, newHash string) error {
	query := "UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2"
	result, err := getQuerier(r.db, tx).Exec(ctx, query, newHash, userID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *PostgresUserRepository) UpdateStatus(ctx context.Context, tx db.Transaction, userID int64, status UserStatus) error {
	query := "UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2"
	result, err := getQuerier(r.db, tx).Exec(ctx, query, status, userID)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func scanUser(scanner db.Scanner) (*User, error) {
	var user User
	var phone sql.NullString

	err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&phone,
		&user.PasswordHash,
		&user.Role,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if phone.Valid {
		user.Phone = &phone.String
	}
	return &user, nil
}
