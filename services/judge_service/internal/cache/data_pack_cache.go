package cache

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"fuzoj/internal/common/cache"
	"fuzoj/internal/common/storage"
	appErr "fuzoj/pkg/errors"
	"fuzoj/services/judge_service/internal/pmodel"
)

const (
	metaFileName  = "meta.json"
	tempFileName  = "data-pack.tmp"
	lockKeyPrefix = "judge:datapack:lock:"
)

type cacheEntry struct {
	key       string
	path      string
	sizeBytes int64
	expiresAt time.Time
}

// DataPackCache manages local data pack caching.
type DataPackCache struct {
	rootDir    string
	ttl        time.Duration
	lockWait   time.Duration
	maxEntries int
	maxBytes   int64
	bucket     string
	storage    storage.ObjectStorage
	lock       cache.LockOps
	mu         sync.Mutex
	entries    map[string]*cacheEntry
	lruKeys    []string
	totalSize  int64
}

// NewDataPackCache creates a new cache.
func NewDataPackCache(rootDir string, ttl time.Duration, lockWait time.Duration, maxEntries int, maxBytes int64, bucket string, storageClient storage.ObjectStorage, lock cache.LockOps) *DataPackCache {
	if maxEntries <= 0 {
		maxEntries = 64
	}
	if lockWait <= 0 {
		lockWait = 30 * time.Second
	}
	return &DataPackCache{
		rootDir:    rootDir,
		ttl:        ttl,
		lockWait:   lockWait,
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		bucket:     bucket,
		storage:    storageClient,
		lock:       lock,
		entries:    make(map[string]*cacheEntry),
	}
}

// Get returns the local cache path for a problem data pack.
func (c *DataPackCache) Get(ctx context.Context, meta pmodel.ProblemMeta) (string, error) {
	if meta.ProblemID <= 0 || meta.Version <= 0 {
		return "", appErr.ValidationError("problem_id", "required")
	}
	if c.storage == nil {
		return "", appErr.New(appErr.CacheError).WithMessage("storage client is not initialized")
	}
	if c.rootDir == "" {
		return "", appErr.New(appErr.CacheError).WithMessage("cache root is not configured")
	}
	key := cacheKey(meta.ProblemID, meta.Version)
	path := filepath.Join(c.rootDir, fmt.Sprintf("%d", meta.ProblemID), fmt.Sprintf("%d", meta.Version))

	if ok := c.hitEntry(key, meta); ok {
		return path, nil
	}

	if ok := c.checkDisk(path, meta); ok {
		c.addEntry(key, path)
		return path, nil
	}

	if err := c.fetchAndExtract(ctx, meta, path); err != nil {
		return "", err
	}
	c.addEntry(key, path)
	return path, nil
}

func (c *DataPackCache) hitEntry(key string, meta pmodel.ProblemMeta) bool {
	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return false
	}
	if time.Now().After(entry.expiresAt) {
		c.removeEntryLocked(key)
		c.mu.Unlock()
		return false
	}
	entry.expiresAt = time.Now().Add(c.ttl)
	c.touchLocked(key)
	c.mu.Unlock()
	return true
}

func (c *DataPackCache) checkDisk(path string, meta pmodel.ProblemMeta) bool {
	metaPath := filepath.Join(path, metaFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return false
	}
	var stored pmodel.ProblemMeta
	if err := json.Unmarshal(data, &stored); err != nil {
		return false
	}
	if stored.ManifestHash != meta.ManifestHash || stored.DataPackHash != meta.DataPackHash {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "manifest.json")); err != nil {
		return false
	}
	return true
}

func (c *DataPackCache) fetchAndExtract(ctx context.Context, meta pmodel.ProblemMeta, path string) error {
	if c.lock == nil {
		return appErr.New(appErr.CacheError).WithMessage("lock client is not initialized")
	}
	lockKey := lockKeyPrefix + cacheKey(meta.ProblemID, meta.Version)
	locked, err := c.lock.TryLock(ctx, lockKey, 5*time.Minute)
	if err != nil {
		return appErr.Wrapf(err, appErr.LockFailed, "acquire data pack lock failed")
	}
	if !locked {
		return c.waitForCache(ctx, meta, path)
	}
	defer func() {
		_ = c.lock.Unlock(ctx, lockKey)
	}()

	if ok := c.checkDisk(path, meta); ok {
		return nil
	}

	if err := os.RemoveAll(path); err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "cleanup cache dir failed")
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "create cache dir failed")
	}

	tempPath := filepath.Join(path, tempFileName)
	if err := c.downloadDataPack(ctx, meta, tempPath); err != nil {
		return err
	}
	if err := extractDataPack(tempPath, path); err != nil {
		return err
	}
	_ = os.Remove(tempPath)

	metaBytes, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(path, metaFileName), metaBytes, 0644); err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "write meta failed")
	}
	return nil
}

func (c *DataPackCache) waitForCache(ctx context.Context, meta pmodel.ProblemMeta, path string) error {
	deadline := time.Now().Add(c.lockWait)
	for {
		if ok := c.checkDisk(path, meta); ok {
			return nil
		}
		if time.Now().After(deadline) {
			return appErr.New(appErr.Timeout).WithMessage("wait for data pack cache timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (c *DataPackCache) downloadDataPack(ctx context.Context, meta pmodel.ProblemMeta, dstPath string) error {
	if meta.DataPackKey == "" {
		return appErr.ValidationError("data_pack_key", "required")
	}
	reader, err := c.storage.GetObject(ctx, c.bucket, meta.DataPackKey)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "download data pack failed")
	}
	defer reader.Close()

	file, err := os.Create(dstPath)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "create data pack file failed")
	}
	defer file.Close()

	hasher := sha256.New()
	tee := io.TeeReader(reader, hasher)
	if _, err := io.Copy(file, tee); err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "write data pack file failed")
	}
	if meta.DataPackHash != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, meta.DataPackHash) {
			return appErr.New(appErr.CacheError).WithMessage("data pack hash mismatch")
		}
	}
	return nil
}

func extractDataPack(srcPath, dstDir string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "open data pack failed")
	}
	defer file.Close()

	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return appErr.Wrapf(err, appErr.CacheError, "create zstd reader failed")
	}
	defer zstdReader.Close()

	tr := tar.NewReader(zstdReader)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return appErr.Wrapf(err, appErr.CacheError, "read tar entry failed")
		}
		if hdr.Name == "" {
			continue
		}
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return appErr.New(appErr.CacheError).WithMessage("invalid tar entry path")
		}
		target := filepath.Join(dstDir, cleanName)
		if !strings.HasPrefix(target, filepath.Clean(dstDir)+string(filepath.Separator)) {
			return appErr.New(appErr.CacheError).WithMessage("tar entry escape detected")
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return appErr.Wrapf(err, appErr.CacheError, "create dir failed")
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return appErr.Wrapf(err, appErr.CacheError, "create parent dir failed")
			}
			file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(hdr.Mode))
			if err != nil {
				return appErr.Wrapf(err, appErr.CacheError, "create file failed")
			}
			if _, err := io.Copy(file, tr); err != nil {
				_ = file.Close()
				return appErr.Wrapf(err, appErr.CacheError, "write file failed")
			}
			_ = file.Close()
		default:
			// skip other types
		}
	}
	return nil
}

func (c *DataPackCache) addEntry(key, path string) {
	size := dirSize(path)
	c.mu.Lock()
	if existing, ok := c.entries[key]; ok {
		c.totalSize -= existing.sizeBytes
	}
	c.entries[key] = &cacheEntry{
		key:       key,
		path:      path,
		sizeBytes: size,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.totalSize += size
	c.touchLocked(key)
	c.evictLocked()
	c.mu.Unlock()
}

func (c *DataPackCache) touchLocked(key string) {
	for i, k := range c.lruKeys {
		if k == key {
			c.lruKeys = append(c.lruKeys[:i], c.lruKeys[i+1:]...)
			break
		}
	}
	c.lruKeys = append(c.lruKeys, key)
}

func (c *DataPackCache) evictLocked() {
	for {
		if c.maxEntries > 0 && len(c.entries) > c.maxEntries {
			c.removeOldestLocked()
			continue
		}
		if c.maxBytes > 0 && c.totalSize > c.maxBytes {
			c.removeOldestLocked()
			continue
		}
		break
	}
}

func (c *DataPackCache) removeOldestLocked() {
	if len(c.lruKeys) == 0 {
		return
	}
	key := c.lruKeys[0]
	c.lruKeys = c.lruKeys[1:]
	c.removeEntryLocked(key)
}

func (c *DataPackCache) removeEntryLocked(key string) {
	entry, ok := c.entries[key]
	if !ok {
		return
	}
	delete(c.entries, key)
	c.totalSize -= entry.sizeBytes
	_ = os.RemoveAll(entry.path)
}

func cacheKey(problemID int64, version int32) string {
	return fmt.Sprintf("%d:%d", problemID, version)
}

func dirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
