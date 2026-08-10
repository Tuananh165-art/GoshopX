package internal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Tuananh165art/GoshopX/admin/models"
	"github.com/redis/go-redis/v9"
)

type DashboardCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewDashboardCache(client *redis.Client, ttl time.Duration) *DashboardCache {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &DashboardCache{client: client, ttl: ttl}
}

func (c *DashboardCache) Get(ctx context.Context, key string) (*models.Dashboard, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	var result models.Dashboard
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *DashboardCache) Set(ctx context.Context, key string, dashboard models.Dashboard) error {
	value, err := json.Marshal(dashboard)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, value, c.ttl).Err()
}

func (c *DashboardCache) Invalidate(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, "admin:dashboard:*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}
