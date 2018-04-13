package redis

import (
	"fmt"

	"github.com/go-redis/redis"
)

type Client struct {
	DB *redis.Client
}

func main() {
	fmt.Println("vim-go")
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		Password: "",
		DB: 0,
	})

	fmt.Println("Pinging client")
	pong, err := client.Ping().Result()
	fmt.Println(pong, err)

	err = client.Set("test", "1234", 0).Err()
	if err != nil {
		panic(err)
	}

	val, err2 := client.Get("test").Result()
	if err2 != nil {
		panic(err2)
	}

	fmt.Println("should equal 1234:", val)
}

func New () (*Client, error) {
	return &Client {
		DB: redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
			Password: "",
			DB: 0,
		}),
	}, nil
}

func (client *Client) Get (key string) (string) {
	cache, err := New()
	if err != nil {
		fmt.Println(err)
	}
	result, _ := cache.DB.Get(key).Result()
	return result
}
