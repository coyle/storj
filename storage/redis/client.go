package redis

import (
	"github.com/go-redis/redis"
)

// Client is the entrypoint into Redis
type Client struct {
	DB *redis.Client
}

// New returns a configured Client instance, verifying a sucessful connection to redis
func New(address, password string, db int) (*Client, error) {
	c := &Client{
		DB: redis.NewClient(&redis.Options{
			Addr:     address,
			Password: password,
			DB:       db,
		}),
	}

	// ping here to verify we are able to connect to the redis instacne with the initialized client.
	if err := c.DB.Ping().Err(); err != nil {
		return nil, err
	}

	return c, nil
}

// Get looks up the provided key from the redis cache returning either an error or the result.
func (c *Client) Get(key string) (string, error) {
	return c.DB.Get(key).Result()
}

// Set adds a value to the provided key in the Redis cache, returning an error on failure.
func (c *Client) Set(key, value string) error {
	return c.DB.Set(key, value, 0).Err()
}
