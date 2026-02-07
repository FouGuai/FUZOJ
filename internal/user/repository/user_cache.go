package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"fuzoj/internal/common/cache"
)

const (
	userInfoKeyPrefix     = "user:info:"
	userUsernameKeyPrefix = "user:username:"
	userEmailKeyPrefix    = "user:email:"

	defaultUserCacheTTL      = 30 * time.Minute
	defaultUserCacheEmptyTTL = 5 * time.Minute
)

type UserCacheRepository interface {
	GetByID(ctx context.Context, id int64, loader func(context.Context) (*User, error)) (*User, error)
	GetByUsername(ctx context.Context, username string, loader func(context.Context) (*User, error)) (*User, error)
	GetByEmail(ctx context.Context, email string, loader func(context.Context) (*User, error)) (*User, error)
	Set(ctx context.Context, user *User) error
	Delete(ctx context.Context, user *User) error
	DeleteByID(ctx context.Context, id int64) error
	DeleteByUsername(ctx context.Context, username string) error
	DeleteByEmail(ctx context.Context, email string) error
}

type RedisUserCacheRepository struct {
	cache    cache.Cache
	ttl      time.Duration
	emptyTTL time.Duration
}

func NewUserCacheRepository(cacheClient cache.Cache) UserCacheRepository {
	return NewUserCacheRepositoryWithTTL(cacheClient, defaultUserCacheTTL, defaultUserCacheEmptyTTL)
}

func NewUserCacheRepositoryWithTTL(cacheClient cache.Cache, ttl, emptyTTL time.Duration) UserCacheRepository {
	return &RedisUserCacheRepository{cache: cacheClient, ttl: ttl, emptyTTL: emptyTTL}
}

func (r *RedisUserCacheRepository) GetByID(ctx context.Context, id int64, loader func(context.Context) (*User, error)) (*User, error) {
	if r.cache == nil {
		return nil, errors.New("cache is nil")
	}

	key := userInfoKey(id)
	return cache.GetWithCached[*User](
		ctx,
		r.cache,
		key,
		r.ttl,
		r.emptyTTL,
		func(user *User) bool { return user == nil },
		marshalUser,
		unmarshalUser,
		loader,
	)
}

func (r *RedisUserCacheRepository) GetByUsername(ctx context.Context, username string, loader func(context.Context) (*User, error)) (*User, error) {
	if r.cache == nil {
		return nil, errors.New("cache is nil")
	}

	key := userUsernameKey(username)
	return cache.GetWithCached[*User](
		ctx,
		r.cache,
		key,
		r.ttl,
		r.emptyTTL,
		func(user *User) bool { return user == nil },
		marshalUser,
		unmarshalUser,
		loader,
	)
}

func (r *RedisUserCacheRepository) GetByEmail(ctx context.Context, email string, loader func(context.Context) (*User, error)) (*User, error) {
	if r.cache == nil {
		return nil, errors.New("cache is nil")
	}

	key := userEmailKey(email)
	return cache.GetWithCached[*User](
		ctx,
		r.cache,
		key,
		r.ttl,
		r.emptyTTL,
		func(user *User) bool { return user == nil },
		marshalUser,
		unmarshalUser,
		loader,
	)
}

func (r *RedisUserCacheRepository) Set(ctx context.Context, user *User) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	if user == nil {
		return errors.New("user is nil")
	}

	payload, err := json.Marshal(user)
	if err != nil {
		return err
	}
	data := string(payload)

	if err := r.cache.Set(ctx, userInfoKey(user.ID), data, r.ttl); err != nil {
		return err
	}
	if user.Username != "" {
		if err := r.cache.Set(ctx, userUsernameKey(user.Username), data, r.ttl); err != nil {
			return err
		}
	}
	if user.Email != "" {
		if err := r.cache.Set(ctx, userEmailKey(user.Email), data, r.ttl); err != nil {
			return err
		}
	}

	return nil
}

func (r *RedisUserCacheRepository) Delete(ctx context.Context, user *User) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	if user == nil {
		return errors.New("user is nil")
	}

	keys := make([]string, 0, 3)
	if user.ID != 0 {
		keys = append(keys, userInfoKey(user.ID))
	}
	if user.Username != "" {
		keys = append(keys, userUsernameKey(user.Username))
	}
	if user.Email != "" {
		keys = append(keys, userEmailKey(user.Email))
	}
	if len(keys) == 0 {
		return nil
	}
	return r.cache.Del(ctx, keys...)
}

func (r *RedisUserCacheRepository) DeleteByID(ctx context.Context, id int64) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	return r.cache.Del(ctx, userInfoKey(id))
}

func (r *RedisUserCacheRepository) DeleteByUsername(ctx context.Context, username string) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	return r.cache.Del(ctx, userUsernameKey(username))
}

func (r *RedisUserCacheRepository) DeleteByEmail(ctx context.Context, email string) error {
	if r.cache == nil {
		return errors.New("cache is nil")
	}
	return r.cache.Del(ctx, userEmailKey(email))
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
