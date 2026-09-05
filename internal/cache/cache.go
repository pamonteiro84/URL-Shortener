package cache

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
)

func Connect() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
