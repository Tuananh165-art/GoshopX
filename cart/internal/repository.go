package internal

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/redis/go-redis/v9"
)

var ErrCartNotFound = errors.New("cart not found")

type Repository interface {
	GetCart(ctx context.Context, accountID uint64) (*models.Cart, error)
	SaveCart(ctx context.Context, cart *models.Cart, ttl time.Duration) error
	DeleteCart(ctx context.Context, accountID uint64) error
}

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(redisURL string) (Repository, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &redisRepository{client: redis.NewClient(options)}, nil
}

func cartKey(accountID uint64) string {
	return "cart:" + strconv.FormatUint(accountID, 10)
}

func (repository *redisRepository) GetCart(ctx context.Context, accountID uint64) (*models.Cart, error) {
	payload, err := repository.client.Get(ctx, cartKey(accountID)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCartNotFound
	}
	if err != nil {
		return nil, err
	}

	var cart models.Cart
	if err := json.Unmarshal([]byte(payload), &cart); err != nil {
		return nil, err
	}
	return &cart, nil
}

func (repository *redisRepository) SaveCart(ctx context.Context, cart *models.Cart, ttl time.Duration) error {
	body, err := json.Marshal(cart)
	if err != nil {
		return err
	}
	return repository.client.Set(ctx, cartKey(cart.AccountID), body, ttl).Err()
}

func (repository *redisRepository) DeleteCart(ctx context.Context, accountID uint64) error {
	return repository.client.Del(ctx, cartKey(accountID)).Err()
}

