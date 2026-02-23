package judge_service

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

func newFakeCache(t *testing.T) *redis.Redis {
	t.Helper()

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	t.Cleanup(server.Close)

	client, err := redis.NewRedis(redis.RedisConf{
		Host: server.Addr(),
		Type: redis.NodeType,
	})
	if err != nil {
		t.Fatalf("init redis client failed: %v", err)
	}
	return client
}
