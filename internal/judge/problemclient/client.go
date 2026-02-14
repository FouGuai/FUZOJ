package problemclient

import (
	"context"

	problemv1 "fuzoj/api/gen/problem/v1"
	"fuzoj/internal/judge/model"
)

// Client provides problem meta queries.
type Client struct {
	grpc problemv1.ProblemServiceClient
}

// NewClient creates a new client.
func NewClient(grpc problemv1.ProblemServiceClient) *Client {
	return &Client{grpc: grpc}
}

// GetLatest returns latest published meta for a problem.
func (c *Client) GetLatest(ctx context.Context, problemID int64) (model.ProblemMeta, error) {
	resp, err := c.grpc.GetLatest(ctx, &problemv1.GetLatestRequest{ProblemId: problemID})
	if err != nil {
		return model.ProblemMeta{}, err
	}
	meta := resp.GetMeta()
	if meta == nil {
		return model.ProblemMeta{}, nil
	}
	return model.ProblemMeta{
		ProblemID:    meta.ProblemId,
		Version:      meta.Version,
		ManifestHash: meta.ManifestHash,
		DataPackKey:  meta.DataPackKey,
		DataPackHash: meta.DataPackHash,
		UpdatedAt:    meta.UpdatedAt,
	}, nil
}
