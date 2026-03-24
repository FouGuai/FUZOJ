// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"database/sql"
	"time"

	"fuzoj/pkg/contest/eligibility"
	contestRepo "fuzoj/pkg/contest/repository"
	"fuzoj/pkg/submit/statuswriter"
	"fuzoj/services/contest_service/internal/config"
	"fuzoj/services/contest_service/internal/consumer"
	rankRepo "fuzoj/services/contest_service/internal/repository"

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
	ContestRepo             contestRepo.ContestRepository
	ContestStore            rankRepo.ContestRepository
	ContestProblemStore     rankRepo.ContestProblemStore
	ContestParticipantStore rankRepo.ContestParticipantStore
	ProblemRepo             contestRepo.ContestProblemRepository
	ParticipantRepo         contestRepo.ContestParticipantRepository
	EligibilityService      *eligibility.Service
	StatusWriter            *statuswriter.FinalStatusWriter
	ContestDispatchQueue    queue.MessageQueue
	ContestDispatchConsumer *consumer.ContestDispatchConsumer
	JudgeFinalQueue         queue.MessageQueue
	JudgeFinalConsumer      *consumer.JudgeFinalConsumer
	MemberProblemRepo       *rankRepo.MemberProblemRepository
	MemberSummaryRepo       *rankRepo.MemberSummaryRepository
	RankOutboxRepo          *rankRepo.RankOutboxRepository
	RankUpdatePusher        *kq.Pusher
	RankOutboxRelay         *consumer.RankOutboxRelay
	JudgeFinalDeadLetter    *kq.Pusher
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

	contestRepository := contestRepo.NewContestRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	contestStoreRepo := rankRepo.NewContestRepository(conn, cacheClient, ttl, emptyTTL)
	contestProblemStore := rankRepo.NewContestProblemStore(conn)
	contestParticipantStore := rankRepo.NewContestParticipantStore(conn)
	problemRepo := contestRepo.NewContestProblemRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	participantRepo := contestRepo.NewContestParticipantRepository(conn, cacheClient, ttl, emptyTTL, localSize, localTTL)
	eligibilityService := eligibility.NewService(contestRepository, problemRepo, participantRepo)

	statusWriter := statuswriter.NewFinalStatusWriter(conn, redisClient, c.ContestDispatch.StatusTTL)
	memberProblemRepo := rankRepo.NewMemberProblemRepository(conn)
	memberSummaryRepo := rankRepo.NewMemberSummaryRepository(conn)
	rankOutboxRepo := rankRepo.NewRankOutboxRepository(conn)

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

	var rankUpdatePusher *kq.Pusher
	if len(c.Kafka.Brokers) > 0 && c.RankUpdate.Topic != "" {
		rankUpdatePusher = kq.NewPusher(c.Kafka.Brokers, c.RankUpdate.Topic, kq.WithSyncPush())
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
		}, consumer.TimeoutConfig{MQ: c.Timeouts.MQ, Cache: c.Timeouts.Cache})
		if c.ContestDispatch.DeadLetterTopic != "" {
			deadLetterPusher = kq.NewPusher(c.Kafka.Brokers, c.ContestDispatch.DeadLetterTopic, kq.WithSyncPush())
			dispatchConsumer.SetDeadLetterPusher(deadLetterPusher)
		}
		kqConf := consumer.BuildContestDispatchKqConf(c)
		dispatchQueue = kq.MustNewQueue(kqConf, dispatchConsumer)
	} else {
		logx.Infof("contest dispatch consumer is disabled due to missing kafka config")
	}

	var judgeFinalConsumer *consumer.JudgeFinalConsumer
	var judgeFinalQueue queue.MessageQueue
	var judgeFinalDeadLetter *kq.Pusher
	if len(c.Kafka.Brokers) > 0 && c.JudgeFinal.Topic != "" {
		judgeFinalConsumer = consumer.NewJudgeFinalConsumer(
			conn,
			redisClient,
			contestRepository,
			eligibilityService,
			memberProblemRepo,
			memberSummaryRepo,
			rankOutboxRepo,
			consumer.JudgeFinalOptions{
				IdempotencyTTL:  c.JudgeFinal.IdempotencyTTL,
				MessageTTL:      c.JudgeFinal.MessageTTL,
				MaxRetries:      c.JudgeFinal.MaxRetries,
				RetryDelay:      c.JudgeFinal.RetryDelay,
				DeadLetterTopic: c.JudgeFinal.DeadLetterTopic,
			},
			consumer.TimeoutConfig{MQ: c.Timeouts.MQ, Cache: c.Timeouts.Cache},
		)
		if c.JudgeFinal.DeadLetterTopic != "" {
			judgeFinalDeadLetter = kq.NewPusher(c.Kafka.Brokers, c.JudgeFinal.DeadLetterTopic, kq.WithSyncPush())
			judgeFinalConsumer.SetDeadLetterPusher(judgeFinalDeadLetter)
		}
		kqConf := consumer.BuildJudgeFinalKqConf(c)
		judgeFinalQueue = kq.MustNewQueue(kqConf, judgeFinalConsumer)
	} else {
		logx.Infof("judge final consumer is disabled due to missing kafka config")
	}

	var rankOutboxRelay *consumer.RankOutboxRelay
	if rankOutboxRepo != nil && len(c.Kafka.Brokers) > 0 && c.RankUpdate.Topic != "" && redisClient != nil {
		rankOutboxRelay = consumer.NewRankOutboxRelay(rankOutboxRepo, redisClient, consumer.RankOutboxRelayOptions{
			KafkaBrokers:        c.Kafka.Brokers,
			RankUpdateTopic:     c.RankUpdate.Topic,
			PublishBatchSize:    c.RankOutbox.ClaimBatchSize,
			PublishBatchTimeout: time.Second,
			ContestScanInterval: c.RankOutbox.ContestScanInterval,
			ContestScanBatch:    c.RankOutbox.ContestScanBatch,
			ClaimBatchSize:      c.RankOutbox.ClaimBatchSize,
			LeaseDuration:       c.RankOutbox.LeaseDuration,
			LeaseRenewInterval:  c.RankOutbox.LeaseRenewInterval,
			RetryBaseDelay:      c.RankOutbox.RetryBaseDelay,
			RetryMaxDelay:       c.RankOutbox.RetryMaxDelay,
			CleanupInterval:     c.RankOutbox.CleanupInterval,
			SentRetention:       c.RankOutbox.SentRetention,
			CleanupBatchSize:    c.RankOutbox.CleanupBatchSize,
			RequeueBatchSize:    c.RankOutbox.RequeueBatchSize,
			DBTimeout:           c.Timeouts.DB,
			MQTimeout:           c.Timeouts.MQ,
		})
	} else if rankOutboxRepo != nil && len(c.Kafka.Brokers) > 0 && c.RankUpdate.Topic != "" && redisClient == nil {
		logx.Infof("rank outbox relay is disabled due to missing redis config")
	}

	return &ServiceContext{
		Config:                  c,
		Conn:                    conn,
		Cache:                   cacheClient,
		Redis:                   redisClient,
		ContestRepo:             contestRepository,
		ContestStore:            contestStoreRepo,
		ContestProblemStore:     contestProblemStore,
		ContestParticipantStore: contestParticipantStore,
		ProblemRepo:             problemRepo,
		ParticipantRepo:         participantRepo,
		EligibilityService:      eligibilityService,
		StatusWriter:            statusWriter,
		ContestDispatchQueue:    dispatchQueue,
		ContestDispatchConsumer: dispatchConsumer,
		JudgeFinalQueue:         judgeFinalQueue,
		JudgeFinalConsumer:      judgeFinalConsumer,
		MemberProblemRepo:       memberProblemRepo,
		MemberSummaryRepo:       memberSummaryRepo,
		RankOutboxRepo:          rankOutboxRepo,
		RankUpdatePusher:        rankUpdatePusher,
		RankOutboxRelay:         rankOutboxRelay,
		JudgeFinalDeadLetter:    judgeFinalDeadLetter,
		DeadLetterPusher:        deadLetterPusher,
		JudgePushers:            pushers,
	}
}
