package svc

import (
	"strings"

	"fuzoj/services/submit_service/internal/consumer"
	"fuzoj/services/submit_service/internal/repository"
)

// ResolveDispatchTarget resolves retry dispatch target by submission scene.
func (s *ServiceContext) ResolveDispatchTarget(record repository.SubmissionDispatchRecord) (string, consumer.MessagePusher) {
	if s == nil {
		return "", nil
	}
	if strings.TrimSpace(record.ContestID) != "" {
		return s.Config.Submit.ContestDispatch.Topic, s.ContestDispatchPusher
	}
	switch strings.ToLower(strings.TrimSpace(record.Scene)) {
	case "contest":
		return s.Config.Topics.Level0, s.TopicPushers.Level0
	case "custom":
		return s.Config.Topics.Level2, s.TopicPushers.Level2
	case "rejudge":
		return s.Config.Topics.Level3, s.TopicPushers.Level3
	default:
		return s.Config.Topics.Level1, s.TopicPushers.Level1
	}
}
