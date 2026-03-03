// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"database/sql"

	"fuzoj/pkg/contest/eligibility"
	"fuzoj/pkg/contest/repository"
	"fuzoj/pkg/submit/statuswriter"
	"fuzoj/services/contest_service/internal/config"
	"fuzoj/services/contest_service/internal/consumer"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/syncx"
)

type ServiceContext struct {
	Config                  config.Config
	Conn                    sqlx.SqlConn
	Cache                   cache.Cache
	Redis                   *redis.Redis
	ContestRepo             repository.ContestRepository
	ProblemRepo             repository.ContestProblemRepository
	ParticipantRepo         repository.ContestParticipantRepository
	EligibilityService      *eligibility.Service
	StatusWriter            *statuswriter.FinalStatusWriter
	ContestDispatchQueue    queue.MessageQueue
	ContestDispatchConsumer *consumer.ContestDispatchConsumer
	DeadLetterPusher        *kq.Pusher
	JudgePushers            TopicPushers
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	var cacheClient cache.Cache
	if len(c.Cache) > 0 {
		cacheClient = cache.New(c.Cache, syncx.NewSingleFlight(), cache.NewStat("contest"), sql.ErrNoRows)
	}

	var redisClient *redis.Redis
	if c.Redis.Host != "" {
		redisClient = redis.MustNewRedis(c.Redis)
	}

	ttl := c.Contest.EligibilityCacheTTL
	emptyTTL := c.Contest.EligibilityEmptyTTL
	localSize := c.Contest.EligibilityLocalCacheSize
	localTTL := c.Contest.EligibilityLocalCacheTTL
	if localTTL <= 0 {
		localTTL = ttl
	}
	if localSize == 0 {
		localSize = 1024
	}

	contestRepo := repository.NewContestRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	problemRepo := repository.NewContestProblemRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	participantRepo := repository.NewContestParticipantRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	eligibilityService := eligibility.NewService(contestRepo, problemRepo, participantRepo)

	statusWriter := statuswriter.NewFinalStatusWriter(conn, redisClient, c.ContestDispatch.StatusTTL)

	pushers := TopicPushers{}
	if len(c.Kafka.Brokers) > 0 {
		if c.Topics.Level0 != "" {
			pushers.Level0 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level0, kq.WithSyncPush())
		}
		if c.Topics.Level1 != "" {
			pushers.Level1 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level1, kq.WithSyncPush())
		}
		if c.Topics.Level2 != "" {
			pushers.Level2 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level2, kq.WithSyncPush())
		}
		if c.Topics.Level3 != "" {
			pushers.Level3 = kq.NewPusher(c.Kafka.Brokers, c.Topics.Level3, kq.WithSyncPush())
		}
	}

	var dispatchConsumer *consumer.ContestDispatchConsumer
	var dispatchQueue queue.MessageQueue
	var deadLetterPusher *kq.Pusher
	if len(c.Kafka.Brokers) > 0 && c.ContestDispatch.Topic != "" {
		dispatchConsumer = consumer.NewContestDispatchConsumer(eligibilityService, statusWriter, redisClient, pushers.Level0, consumer.DispatchOptions{
			Topic:           c.ContestDispatch.Topic,
			IdempotencyTTL:  c.ContestDispatch.IdempotencyTTL,
			MessageTTL:      c.ContestDispatch.MessageTTL,
			MaxRetries:      c.ContestDispatch.MaxRetries,
			RetryDelay:      c.ContestDispatch.RetryDelay,
			DeadLetterTopic: c.ContestDispatch.DeadLetterTopic,
		}, consumer.TimeoutConfig{MQ: c.Timeouts.MQ})
		if c.ContestDispatch.DeadLetterTopic != "" {
			deadLetterPusher = kq.NewPusher(c.Kafka.Brokers, c.ContestDispatch.DeadLetterTopic, kq.WithSyncPush())
			dispatchConsumer.SetDeadLetterPusher(deadLetterPusher)
		}
		kqConf := consumer.BuildContestDispatchKqConf(c)
		dispatchQueue = kq.MustNewQueue(kqConf, dispatchConsumer)
	} else {
		logx.Infof("contest dispatch consumer is disabled due to missing kafka config")
	}

	return &ServiceContext{
		Config:                  c,
		Conn:                    conn,
		Cache:                   cacheClient,
		Redis:                   redisClient,
		ContestRepo:             contestRepo,
		ProblemRepo:             problemRepo,
		ParticipantRepo:         participantRepo,
		EligibilityService:      eligibilityService,
		StatusWriter:            statusWriter,
		ContestDispatchQueue:    dispatchQueue,
		ContestDispatchConsumer: dispatchConsumer,
		DeadLetterPusher:        deadLetterPusher,
		JudgePushers:            pushers,
	}
}
