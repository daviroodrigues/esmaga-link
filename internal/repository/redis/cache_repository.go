package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CacheRepository struct {
	client *redis.Client
}

func NewCacheRepository(client *redis.Client) *CacheRepository {
	return &CacheRepository{client: client}
}

func (r *CacheRepository) GetLink(ctx context.Context, shortCode string) (string, error) {
	key := fmt.Sprintf("esmaga:link:%s", shortCode)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("erro ao buscar link no redis: %w", err)
	}
	return val, nil
}

func (r *CacheRepository) SetLink(ctx context.Context, shortCode string, originalURL string, ttl time.Duration) error {
	key := fmt.Sprintf("esmaga:link:%s", shortCode)
	err := r.client.Set(ctx, key, originalURL, ttl).Err()
	if err != nil {
		return fmt.Errorf("erro ao salvar link no redis: %w", err)
	}
	return nil
}

const CacheInvalidateChannel = "esmaga:cache:invalidate"

func (r *CacheRepository) DeleteLink(ctx context.Context, shortCode string) error {
	key := fmt.Sprintf("esmaga:link:%s", shortCode)
	return r.client.Del(ctx, key).Err()
}

// PublishInvalidation notifica todas as réplicas da API para removerem o shortCode do cache L1.
func (r *CacheRepository) PublishInvalidation(ctx context.Context, shortCode string) error {
	return r.client.Publish(ctx, CacheInvalidateChannel, shortCode).Err()
}

// SubscribeInvalidations escuta o canal de invalidação no Redis até que ctx seja cancelado.
func (r *CacheRepository) SubscribeInvalidations(ctx context.Context, onInvalidate func(shortCode string)) {
	pubsub := r.client.Subscribe(ctx, CacheInvalidateChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Payload != "" && onInvalidate != nil {
				onInvalidate(msg.Payload)
			}
		}
	}
}
