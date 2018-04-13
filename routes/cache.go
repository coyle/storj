package routes

import (
	"storj.io/storj/storage/redis"
	"github.com/kataras/iris"
)

type Cache struct {
	DB *redis.Client
}

type Node struct {
	Addr string
	Port int
	NodeID string
}

type Response struct {
	Payload string
	Meta struct {
		Total int
		Offset int
	}
}

func (cache *Cache) Set (ctx iris.Context) {
	node := ctx.Params().Get("node")
	addr := ctx.Params().Get("addr")
	cache.DB.Set(node, addr)
	ctx.Writef(node, addr)
}

func (cache *Cache) GetNodeAddress (ctx iris.Context) {
	key := ctx.Params().Get("node")	
	response := &Response{
		Payload: cache.DB.Get(key),
	}

	response.Meta.Total = 1
	response.Meta.Offset = 0

	ctx.JSON(response)
}
