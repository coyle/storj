package routes

import (
	"storj.io/storj/storage/redis"
	"github.com/kataras/iris"
)

type Cache struct {
	DB *redis.Client
}

func (cache *Cache) Set (ctx iris.Context) {
	node := ctx.Params().Get("node")
	addr := ctx.Params().Get("addr")
	cache.DB.Set(node, addr)
	ctx.Writef(node, addr)
}

func (cache *Cache) GetNodeAddress (ctx iris.Context) {
	key := ctx.Params().Get("node")	
	ctx.Writef(cache.DB.Get(key))
}
