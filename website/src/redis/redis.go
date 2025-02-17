package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

var (
	ErrNil = redis.Nil
	Ctx    = context.Background()
)

func InitRedis(address string) (*Redis, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: "",
		DB:       0,
	})

	if err := client.Ping(Ctx).Err(); err != nil {
		return nil, err
	}

	return &Redis{Client: client}, nil
}
