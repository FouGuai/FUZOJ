package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/db"

	"gopkg.in/yaml.v3"
)

// ConfigSource defines a pluggable source for user service configuration.
type ConfigSource interface {
	LoadUserServiceConfig(ctx context.Context) (UserServiceConfig, error)
}

// UserServiceConfig aggregates database and redis configuration.
type UserServiceConfig struct {
	Database DatabaseConfig
	Redis    RedisConfig
}

// DatabaseConfig mirrors PostgreSQL settings for external configuration.
type DatabaseConfig struct {
	DSN                string        `yaml:"dsn"`
	MaxOpenConnections *int          `yaml:"maxOpenConnections"`
	MaxIdleConnections *int          `yaml:"maxIdleConnections"`
	ConnMaxLifetime    *timeDuration `yaml:"connMaxLifetime"`
	ConnMaxIdleTime    *timeDuration `yaml:"connMaxIdleTime"`
}

// RedisConfig mirrors redis settings for external configuration.
type RedisConfig struct {
	Addr            string        `yaml:"addr"`
	Password        string        `yaml:"password"`
	DB              *int          `yaml:"db"`
	MaxRetries      *int          `yaml:"maxRetries"`
	MinRetryBackoff *timeDuration `yaml:"minRetryBackoff"`
	MaxRetryBackoff *timeDuration `yaml:"maxRetryBackoff"`
	DialTimeout     *timeDuration `yaml:"dialTimeout"`
	ReadTimeout     *timeDuration `yaml:"readTimeout"`
	WriteTimeout    *timeDuration `yaml:"writeTimeout"`
	PoolSize        *int          `yaml:"poolSize"`
	MinIdleConns    *int          `yaml:"minIdleConns"`
	PoolTimeout     *timeDuration `yaml:"poolTimeout"`
	ConnMaxIdleTime *timeDuration `yaml:"connMaxIdleTime"`
	ConnMaxLifetime *timeDuration `yaml:"connMaxLifetime"`
}

// timeDuration wraps time.Duration for YAML unmarshalling.
type timeDuration struct {
	value time.Duration
}

// UnmarshalYAML supports duration strings like "5s" or "2m".
func (d *timeDuration) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	var raw string
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("decode duration failed: %w", err)
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration failed: %w", err)
	}
	d.value = parsed
	return nil
}

// Duration returns the underlying time.Duration.
func (d *timeDuration) Duration() time.Duration {
	if d == nil {
		return 0
	}
	return d.value
}

// FileConfigSource loads configuration from local YAML files.
type FileConfigSource struct {
	DatabasePath string
	RedisPath    string
}

// NewFileConfigSource creates a file-based config source.
func NewFileConfigSource(databasePath, redisPath string) *FileConfigSource {
	return &FileConfigSource{
		DatabasePath: databasePath,
		RedisPath:    redisPath,
	}
}

// LoadUserServiceConfig reads database and redis configuration from files.
func (f *FileConfigSource) LoadUserServiceConfig(ctx context.Context) (UserServiceConfig, error) {
	if f == nil {
		return UserServiceConfig{}, errors.New("config source cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return UserServiceConfig{}, fmt.Errorf("context error before config load: %w", err)
	}
	if f.DatabasePath == "" || f.RedisPath == "" {
		return UserServiceConfig{}, errors.New("config file paths cannot be empty")
	}

	var cfg UserServiceConfig
	if err := readYAMLFile(f.DatabasePath, &cfg.Database); err != nil {
		return UserServiceConfig{}, fmt.Errorf("load database config failed: %w", err)
	}
	if err := readYAMLFile(f.RedisPath, &cfg.Redis); err != nil {
		return UserServiceConfig{}, fmt.Errorf("load redis config failed: %w", err)
	}
	return cfg, nil
}

// Dependencies contains initialized infrastructure for the user service.
type Dependencies struct {
	Database db.Database
	Cache    cache.Cache
}

// Close releases initialized resources.
func (d *Dependencies) Close() error {
	if d == nil {
		return nil
	}
	var errs []error
	if d.Cache != nil {
		if err := d.Cache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close redis cache failed: %w", err))
		}
	}
	if d.Database != nil {
		if err := d.Database.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close database failed: %w", err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

// InitUserService initializes database and redis clients from the given config source.
func InitUserService(ctx context.Context, source ConfigSource) (*Dependencies, error) {
	if source == nil {
		return nil, errors.New("config source cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context error before initialization: %w", err)
	}

	cfg, err := source.LoadUserServiceConfig(ctx)
	if err != nil {
		return nil, err
	}

	dbConfig, err := cfg.Database.toPostgreSQLConfig()
	if err != nil {
		return nil, err
	}
	database, err := db.NewPostgreSQLWithConfig(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("init database failed: %w", err)
	}

	redisConfig, err := cfg.Redis.toRedisConfig()
	if err != nil {
		_ = database.Close()
		return nil, err
	}
	redisCache, err := cache.NewRedisCacheWithConfig(redisConfig)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("init redis failed: %w", err)
	}

	return &Dependencies{
		Database: database,
		Cache:    redisCache,
	}, nil
}

func (c DatabaseConfig) toPostgreSQLConfig() (*db.PostgreSQLConfig, error) {
	if c.DSN == "" {
		return nil, errors.New("database DSN cannot be empty")
	}

	config := db.DefaultPostgreSQLConfig()
	config.DSN = c.DSN
	if c.MaxOpenConnections != nil {
		config.MaxOpenConnections = *c.MaxOpenConnections
	}
	if c.MaxIdleConnections != nil {
		config.MaxIdleConnections = *c.MaxIdleConnections
	}
	if c.ConnMaxLifetime != nil {
		config.ConnMaxLifetime = c.ConnMaxLifetime.Duration()
	}
	if c.ConnMaxIdleTime != nil {
		config.ConnMaxIdleTime = c.ConnMaxIdleTime.Duration()
	}
	return config, nil
}

func (c RedisConfig) toRedisConfig() (*cache.RedisConfig, error) {
	if c.Addr == "" {
		return nil, errors.New("redis addr cannot be empty")
	}

	config := cache.DefaultRedisConfig()
	config.Addr = c.Addr
	config.Password = c.Password
	if c.DB != nil {
		config.DB = *c.DB
	}
	if c.MaxRetries != nil {
		config.MaxRetries = *c.MaxRetries
	}
	if c.MinRetryBackoff != nil {
		config.MinRetryBackoff = c.MinRetryBackoff.Duration()
	}
	if c.MaxRetryBackoff != nil {
		config.MaxRetryBackoff = c.MaxRetryBackoff.Duration()
	}
	if c.DialTimeout != nil {
		config.DialTimeout = c.DialTimeout.Duration()
	}
	if c.ReadTimeout != nil {
		config.ReadTimeout = c.ReadTimeout.Duration()
	}
	if c.WriteTimeout != nil {
		config.WriteTimeout = c.WriteTimeout.Duration()
	}
	if c.PoolSize != nil {
		config.PoolSize = *c.PoolSize
	}
	if c.MinIdleConns != nil {
		config.MinIdleConns = *c.MinIdleConns
	}
	if c.PoolTimeout != nil {
		config.PoolTimeout = c.PoolTimeout.Duration()
	}
	if c.ConnMaxIdleTime != nil {
		config.ConnMaxIdleTime = c.ConnMaxIdleTime.Duration()
	}
	if c.ConnMaxLifetime != nil {
		config.ConnMaxLifetime = c.ConnMaxLifetime.Duration()
	}
	return config, nil
}

func readYAMLFile(path string, target interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file failed: %w", err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("unmarshal yaml failed: %w", err)
	}
	return nil
}
