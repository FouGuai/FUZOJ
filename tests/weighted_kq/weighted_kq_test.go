package weighted_kq_test

import (
	"testing"

	"fuzoj/internal/common/mq/weighted_kq"
)

func TestBuildWeightedKqConfsProportional(t *testing.T) {
	t.Parallel()
	confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
		Brokers:         []string{"127.0.0.1:9092"},
		Group:           "test-group",
		Topics:          []string{"topic-a", "topic-b"},
		TopicWeights:    map[string]int{"topic-a": 8, "topic-b": 4},
		ConsumersTotal:  6,
		ProcessorsTotal: 6,
		ServiceName:     "judge",
	})
	if err != nil {
		t.Fatalf("build confs failed: %v", err)
	}
	if len(confs) != 2 {
		t.Fatalf("expected 2 confs, got %d", len(confs))
	}
	got := map[string]struct {
		consumers  int
		processors int
	}{}
	for _, conf := range confs {
		got[conf.Topic] = struct {
			consumers  int
			processors int
		}{consumers: conf.Consumers, processors: conf.Processors}
	}
	if got["topic-a"].consumers != 4 || got["topic-a"].processors != 6 {
		t.Fatalf("unexpected topic-a allocation: %+v", got["topic-a"])
	}
	if got["topic-b"].consumers != 2 || got["topic-b"].processors != 6 {
		t.Fatalf("unexpected topic-b allocation: %+v", got["topic-b"])
	}
}

func TestBuildWeightedKqConfsAutoAddRetry(t *testing.T) {
	t.Parallel()
	confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
		Brokers:         []string{"127.0.0.1:9092"},
		Group:           "test-group",
		Topics:          []string{"topic-a"},
		ConsumersTotal:  2,
		ProcessorsTotal: 2,
		RetryTopic:      "topic-retry",
		AutoAddRetry:    true,
	})
	if err != nil {
		t.Fatalf("build confs failed: %v", err)
	}
	if len(confs) != 2 {
		t.Fatalf("expected 2 confs with retry topic, got %d", len(confs))
	}
}

func TestBuildWeightedKqConfsMinAllocation(t *testing.T) {
	t.Parallel()
	confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
		Brokers:         []string{"127.0.0.1:9092"},
		Group:           "test-group",
		Topics:          []string{"topic-a", "topic-b", "topic-c"},
		ConsumersTotal:  1,
		ProcessorsTotal: 1,
	})
	if err != nil {
		t.Fatalf("build confs failed: %v", err)
	}
	if len(confs) != 3 {
		t.Fatalf("expected 3 confs, got %d", len(confs))
	}
	for _, conf := range confs {
		if conf.Consumers != 1 || conf.Processors != 1 {
			t.Fatalf("expected min allocation 1, got %d/%d for %s", conf.Consumers, conf.Processors, conf.Topic)
		}
	}
}

func TestBuildWeightedKqConfsSharedProcessors(t *testing.T) {
	t.Parallel()
	confs, err := weighted_kq.BuildWeightedKqConfs(weighted_kq.WeightedKqOptions{
		Brokers:         []string{"127.0.0.1:9092"},
		Group:           "test-group",
		Topics:          []string{"topic-a", "topic-b", "topic-c"},
		TopicWeights:    map[string]int{"topic-a": 8, "topic-b": 4, "topic-c": 2},
		ConsumersTotal:  14,
		ProcessorsTotal: 12,
	})
	if err != nil {
		t.Fatalf("build confs failed: %v", err)
	}
	for _, conf := range confs {
		if conf.Processors != 12 {
			t.Fatalf("expected shared processors=12, got %d for %s", conf.Processors, conf.Topic)
		}
	}
}
