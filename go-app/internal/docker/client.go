package docker

import (
	"context"

	"github.com/docker/docker/client"
)

type Client struct {
	Client    *client.Client
	limitsFor LimitsProvider
}

func NewClient(host string) (*Client, error) {
	dockerClient, err := client.NewClientWithOpts(
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}
	return &Client{Client: dockerClient}, nil
}

func (c *Client) Close() error {
	if c == nil || c.Client == nil {
		return nil
	}
	return c.Client.Close()
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.Client.Ping(ctx)
	return err
}
