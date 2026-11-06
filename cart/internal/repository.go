package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Tuananh165art/GoshopX/cart/models"
	"github.com/redis/go-redis/v9"
)

var ErrCartNotFound = errors.New("cart not found")
var ErrCartLocked = errors.New("cart is being updated; retry the request")

type Repository interface {
	GetCart(ctx context.Context, accountID uint64) (*models.Cart, error)
	SaveCart(ctx context.Context, cart *models.Cart, ttl time.Duration) error
	DeleteCart(ctx context.Context, accountID uint64) error
	AcquireCartLock(ctx context.Context, accountID uint64, token string, ttl time.Duration) (bool, error)
	ReleaseCartLock(ctx context.Context, accountID uint64, token string) error
	GetCartProposal(ctx context.Context, accountID uint64, idempotencyKey string) (*models.CartProposal, error)
	GetCartProposalByID(ctx context.Context, accountID uint64, proposalID string) (*models.CartProposal, error)
	SaveCartProposal(ctx context.Context, proposal *models.CartProposal, ttl time.Duration) error
	SaveCartAndProposal(ctx context.Context, cart *models.Cart, proposal *models.CartProposal, ttl time.Duration) error
}

type redisRepository struct {
	client *redis.Client
}

func NewRedisRepository(redisURL string) (Repository, error) {
	if !strings.Contains(redisURL, "://") {
		redisURL = "redis://" + redisURL
	}
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &redisRepository{client: redis.NewClient(options)}, nil
}

func cartKey(accountID uint64) string {
	return "cart:" + strconv.FormatUint(accountID, 10)
}

func cartLockKey(accountID uint64) string {
	return fmt.Sprintf("cart-lock:%d", accountID)
}

func cartProposalKey(accountID uint64, suffix string) string {
	return fmt.Sprintf("cart-proposal:%d:%s", accountID, suffix)
}

func (repository *redisRepository) AcquireCartLock(ctx context.Context, accountID uint64, token string, ttl time.Duration) (bool, error) {
	return repository.client.SetNX(ctx, cartLockKey(accountID), token, ttl).Result()
}

func (repository *redisRepository) ReleaseCartLock(ctx context.Context, accountID uint64, token string) error {
	return repository.client.Eval(ctx, `if redis.call('GET', KEYS[1]) == ARGV[1] then return redis.call('DEL', KEYS[1]) else return 0 end`, []string{cartLockKey(accountID)}, token).Err()
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

func (repository *redisRepository) GetCartProposal(ctx context.Context, accountID uint64, idempotencyKey string) (*models.CartProposal, error) {
	return repository.getCartProposal(ctx, cartProposalKey(accountID, "key:"+idempotencyKey))
}

func (repository *redisRepository) GetCartProposalByID(ctx context.Context, accountID uint64, proposalID string) (*models.CartProposal, error) {
	return repository.getCartProposal(ctx, cartProposalKey(accountID, "id:"+proposalID))
}

func (repository *redisRepository) getCartProposal(ctx context.Context, key string) (*models.CartProposal, error) {
	value, err := repository.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrCartNotFound
	}
	if err != nil {
		return nil, err
	}
	var proposal models.CartProposal
	if err := json.Unmarshal(value, &proposal); err != nil {
		return nil, err
	}
	return &proposal, nil
}

func (repository *redisRepository) SaveCartProposal(ctx context.Context, proposal *models.CartProposal, ttl time.Duration) error {
	value, err := json.Marshal(proposal)
	if err != nil {
		return err
	}
	_, err = repository.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, cartProposalKey(proposal.AccountID, "key:"+proposal.IdempotencyKey), value, ttl)
		pipe.Set(ctx, cartProposalKey(proposal.AccountID, "id:"+proposal.ID), value, ttl)
		return nil
	})
	return err
}

func (repository *redisRepository) SaveCartAndProposal(ctx context.Context, cart *models.Cart, proposal *models.CartProposal, ttl time.Duration) error {
	cartValue, err := json.Marshal(cart)
	if err != nil {
		return err
	}
	proposalValue, err := json.Marshal(proposal)
	if err != nil {
		return err
	}
	_, err = repository.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, cartKey(cart.AccountID), cartValue, ttl)
		pipe.Set(ctx, cartProposalKey(proposal.AccountID, "key:"+proposal.IdempotencyKey), proposalValue, ttl)
		pipe.Set(ctx, cartProposalKey(proposal.AccountID, "id:"+proposal.ID), proposalValue, ttl)
		return nil
	})
	return err
}
