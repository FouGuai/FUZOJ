package svc

import (
	"database/sql"
	"time"

	"fuzoj/pkg/contest/eligibility"
	"fuzoj/pkg/contest/repository"
	"fuzoj/services/contest_rpc_service/internal/config"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/core/syncx"
)

type ServiceContext struct {
	Config             config.Config
	Conn               sqlx.SqlConn
	Cache              cache.Cache
	ContestRepo        repository.ContestRepository
	ProblemRepo        repository.ContestProblemRepository
	ParticipantRepo    repository.ContestParticipantRepository
	EligibilityService *eligibility.Service
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.Mysql.DataSource)

	var cacheClient cache.Cache
	if len(c.Cache) > 0 {
		cacheClient = cache.New(c.Cache, syncx.NewSingleFlight(), cache.NewStat("contest.rpc"), sql.ErrNoRows)
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

	return &ServiceContext{
		Config:             c,
		Conn:               conn,
		Cache:              cacheClient,
		ContestRepo:        contestRepo,
		ProblemRepo:        problemRepo,
		ParticipantRepo:    participantRepo,
		EligibilityService: eligibilityService,
	}
}

func EligibilityRequestFromProto(in interface {
	GetContestId() string
	GetUserId() int64
	GetProblemId() int64
}, now time.Time) eligibility.Request {
	return eligibility.Request{
		ContestID: in.GetContestId(),
		UserID:    in.GetUserId(),
		ProblemID: in.GetProblemId(),
		Now:       now,
	}
}
