package metainvalidation

import (
	"context"

	"fuzoj/pkg/problem/metapubsub"

	red "github.com/redis/go-redis/v9"
)

// Publisher broadcasts problem-meta invalidation events.
type Publisher struct {
	client *red.Client
}

func NewPublisher(client *red.Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) PublishProblemMetaInvalidated(ctx context.Context, problemID int64, version int32) error {
	return metapubsub.PublishInvalidation(ctx, p.client, problemID, version)
}

func (p *Publisher) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	return p.client.Close()
}
