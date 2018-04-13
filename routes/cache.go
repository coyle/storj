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
	Payload []*Node
	Meta struct {
		Total int
		Offset int
	}
}

func (res *Response) AddNode (node *Node) []*Node {
	res.Payload = append(res.Payload, node)
	return res.Payload
}

func (cache *Cache) Set (ctx iris.Context) {
	node := ctx.Params().Get("node")
	addr := ctx.Params().Get("addr")
	cache.DB.Set(node, addr)
	ctx.Writef(node, addr)
}

func (cache *Cache) GetNodeAddress (ctx iris.Context) {
	response := &Response{}
	response.AddNode(&Node{
		Addr: "127.0.0.1",
		Port: 3000,
		NodeID: "000000000123457737373",
	})
	response.Meta.Total = 1
	response.Meta.Offset = 0

	ctx.JSON(response)
}
