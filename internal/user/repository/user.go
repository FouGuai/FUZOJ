package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"fuzoj/internal/common/cache"
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

type MySQLUserRepository struct {
	dbProvider db.Provider
	cache      cache.Cache
	ttl        time.Duration
	emptyTTL   time.Duration
}

func NewUserRepository(provider db.Provider, cacheClient cache.Cache) UserRepository {
	return NewUserRepositoryWithTTL(provider, cacheClient, defaultUserCacheTTL, defaultUserCacheEmptyTTL)
}

func NewUserRepositoryWithTTL(provider db.Provider, cacheClient cache.Cache, ttl, emptyTTL time.Duration) UserRepository {
	if ttl <= 0 {
		ttl = defaultUserCacheTTL
	}
	if emptyTTL <= 0 {
		emptyTTL = defaultUserCacheEmptyTTL
	}
	return &MySQLUserRepository{
		dbProvider: provider,
		cache:      cacheClient,
		ttl:        ttl,
		emptyTTL:   emptyTTL,
	}
}

const userColumns = "id, username, email, phone, password_hash, role, status, created_at, updated_at"

func (r *MySQLUserRepository) Create(ctx context.Context, tx db.Transaction, user *User) (int64, error) {
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

	query := "INSERT INTO users (username, email, phone, password_hash, role, status) VALUES (?, ?, ?, ?, ?, ?)"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return 0, err
	}
	result, err := querier.Exec(ctx, query, user.Username, user.Email, phone, user.PasswordHash, role, status)
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
	if r.cache != nil {
		user.ID = id
		r.setCache(ctx, user)
	}
	return id, nil
}

func (r *MySQLUserRepository) GetByID(ctx context.Context, tx db.Transaction, id int64) (*User, error) {
	if r.cache != nil && tx == nil {
		user, err := cache.GetWithCached[*User](
			ctx,
			r.cache,
			userInfoKey(id),
			cache.JitterTTL(r.ttl),
			cache.JitterTTL(r.emptyTTL),
			func(user *User) bool { return user == nil },
			marshalUser,
			unmarshalUser,
			func(ctx context.Context) (*User, error) {
				user, err := r.getByIDFromDB(ctx, nil, id)
				if err != nil {
					if errors.Is(err, ErrUserNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return user, nil
			},
		)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrUserNotFound
		}
		return user, nil
	}
	return r.getByIDFromDB(ctx, tx, id)
}

func (r *MySQLUserRepository) GetByUsername(ctx context.Context, tx db.Transaction, username string) (*User, error) {
	if r.cache != nil && tx == nil {
		user, err := cache.GetWithCached[*User](
			ctx,
			r.cache,
			userUsernameKey(username),
			cache.JitterTTL(r.ttl),
			cache.JitterTTL(r.emptyTTL),
			func(user *User) bool { return user == nil },
			marshalUser,
			unmarshalUser,
			func(ctx context.Context) (*User, error) {
				user, err := r.getByUsernameFromDB(ctx, nil, username)
				if err != nil {
					if errors.Is(err, ErrUserNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return user, nil
			},
		)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrUserNotFound
		}
		return user, nil
	}
	return r.getByUsernameFromDB(ctx, tx, username)
}

func (r *MySQLUserRepository) GetByEmail(ctx context.Context, tx db.Transaction, email string) (*User, error) {
	if r.cache != nil && tx == nil {
		user, err := cache.GetWithCached[*User](
			ctx,
			r.cache,
			userEmailKey(email),
			cache.JitterTTL(r.ttl),
			cache.JitterTTL(r.emptyTTL),
			func(user *User) bool { return user == nil },
			marshalUser,
			unmarshalUser,
			func(ctx context.Context) (*User, error) {
				user, err := r.getByEmailFromDB(ctx, nil, email)
				if err != nil {
					if errors.Is(err, ErrUserNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return user, nil
			},
		)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, ErrUserNotFound
		}
		return user, nil
	}
	return r.getByEmailFromDB(ctx, tx, email)
}

func (r *MySQLUserRepository) ExistsByUsername(ctx context.Context, tx db.Transaction, username string) (bool, error) {
	query := "SELECT 1 FROM users WHERE username = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return false, err
	}
	row := querier.QueryRow(ctx, query, username)
	var one int
	if err := row.Scan(&one); err != nil {
		if db.IsNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MySQLUserRepository) ExistsByEmail(ctx context.Context, tx db.Transaction, email string) (bool, error) {
	query := "SELECT 1 FROM users WHERE email = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return false, err
	}
	row := querier.QueryRow(ctx, query, email)
	var one int
	if err := row.Scan(&one); err != nil {
		if db.IsNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *MySQLUserRepository) UpdatePassword(ctx context.Context, tx db.Transaction, userID int64, newHash string) error {
	var username, email string
	if r.cache != nil {
		var err error
		username, email, err = r.getUserIdentifiers(ctx, tx, userID)
		if err != nil {
			return err
		}
	}
	query := "UPDATE users SET password_hash = ?, updated_at = NOW() WHERE id = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return err
	}
	result, err := querier.Exec(ctx, query, newHash, userID)
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
	if r.cache != nil {
		r.deleteCache(ctx, userID, username, email)
	}
	return nil
}

func (r *MySQLUserRepository) UpdateStatus(ctx context.Context, tx db.Transaction, userID int64, status UserStatus) error {
	var username, email string
	if r.cache != nil {
		var err error
		username, email, err = r.getUserIdentifiers(ctx, tx, userID)
		if err != nil {
			return err
		}
	}
	query := "UPDATE users SET status = ?, updated_at = NOW() WHERE id = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return err
	}
	result, err := querier.Exec(ctx, query, status, userID)
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
	if r.cache != nil {
		r.deleteCache(ctx, userID, username, email)
	}
	return nil
}

const (
	userInfoKeyPrefix     = "user:info:"
	userUsernameKeyPrefix = "user:username:"
	userEmailKeyPrefix    = "user:email:"

	defaultUserCacheTTL      = 30 * time.Minute
	defaultUserCacheEmptyTTL = 5 * time.Minute
)

func (r *MySQLUserRepository) getByIDFromDB(ctx context.Context, tx db.Transaction, id int64) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE id = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return nil, err
	}
	row := querier.QueryRow(ctx, query, id)
	user, err := scanUser(row)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) getByUsernameFromDB(ctx context.Context, tx db.Transaction, username string) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE username = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return nil, err
	}
	row := querier.QueryRow(ctx, query, username)
	user, err := scanUser(row)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) getByEmailFromDB(ctx context.Context, tx db.Transaction, email string) (*User, error) {
	query := "SELECT " + userColumns + " FROM users WHERE email = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return nil, err
	}
	row := querier.QueryRow(ctx, query, email)
	user, err := scanUser(row)
	if err != nil {
		if db.IsNoRows(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *MySQLUserRepository) getUserIdentifiers(ctx context.Context, tx db.Transaction, userID int64) (string, string, error) {
	query := "SELECT username, email FROM users WHERE id = ?"
	querier, err := db.GetProviderQuerier(r.dbProvider, tx)
	if err != nil {
		return "", "", err
	}
	row := querier.QueryRow(ctx, query, userID)
	var username, email string
	if err := row.Scan(&username, &email); err != nil {
		if db.IsNoRows(err) {
			return "", "", ErrUserNotFound
		}
		return "", "", err
	}
	return username, email, nil
}

func (r *MySQLUserRepository) setCache(ctx context.Context, user *User) {
	if r.cache == nil || user == nil {
		return
	}

	payload, err := json.Marshal(user)
	if err != nil {
		return
	}
	data := string(payload)

	_ = r.cache.Set(ctx, userInfoKey(user.ID), data, cache.JitterTTL(r.ttl))
	if user.Username != "" {
		_ = r.cache.Set(ctx, userUsernameKey(user.Username), data, cache.JitterTTL(r.ttl))
	}
	if user.Email != "" {
		_ = r.cache.Set(ctx, userEmailKey(user.Email), data, cache.JitterTTL(r.ttl))
	}
}

func (r *MySQLUserRepository) deleteCache(ctx context.Context, userID int64, username, email string) {
	if r.cache == nil {
		return
	}
	keys := make([]string, 0, 3)
	if userID != 0 {
		keys = append(keys, userInfoKey(userID))
	}
	if username != "" {
		keys = append(keys, userUsernameKey(username))
	}
	if email != "" {
		keys = append(keys, userEmailKey(email))
	}
	if len(keys) == 0 {
		return
	}
	_ = r.cache.Del(ctx, keys...)
}

func userInfoKey(id int64) string {
	return fmt.Sprintf("%s%d", userInfoKeyPrefix, id)
}

func userUsernameKey(username string) string {
	return userUsernameKeyPrefix + username
}

func userEmailKey(email string) string {
	return userEmailKeyPrefix + email
}

func marshalUser(user *User) string {
	payload, err := json.Marshal(user)
	if err != nil {
		return ""
	}
	return string(payload)
}

func unmarshalUser(data string) (*User, error) {
	if data == "" {
		return nil, nil
	}
	var user User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, err
	}
	return &user, nil
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
