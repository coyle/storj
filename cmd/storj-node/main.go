package main

import (
	"fmt"
	"os"
	"log"
	"github.com/kataras/iris"
	"storj.io/storj/routes"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"github.com/urfave/cli"
)

func main() {
	// command line integration
	nodeCli := cli.NewApp()
	nodeCli.Name = "storj-node"
	nodeCli.Version = "0.0.1"
	nodeCli.Commands = []cli.Command{
		{
			Name: "cache",
			Aliases: []string{"c"},
			Usage: "run a redis cache for the node network",
			Action: func (c *cli.Context) error {
				fmt.Println("running as a redis cache node", c.Args().First())
				if err := StartCacheServer(); err != nil {
					return err
				}
				return nil
			},
		},
		{
			Name: "start",
			Aliases: []string{"start", "up", "s"},
			Usage: "start a storj node instance",
			Action: func (c *cli.Context) error {
				if err := StartServer(); err != nil {
					return err
				}
				return nil
			},
		},
	}

	err := nodeCli.Run(os.Args)
  if err != nil {
    log.Fatal(err)
  }
}

func StartServer () error {
	bdb, err := boltdb.New()
	if err != nil {
		fmt.Println(err)
		return err
	}

	defer bdb.DB.Close()

	users := routes.Users{DB: bdb}
	app := iris.Default()

	SetRoutes(app, users)

	app.Run(iris.Addr(":8080"))
	return nil
}

func StartCacheServer () error {
	app := iris.Default()
	rdb, err := redis.New()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(rdb)

	cache := routes.Cache{DB: rdb}
	CacheRoutes(app, cache)

	app.Run(iris.Addr(":9090"))
	return nil
}

// SetRoutes defines all restful routes on the service
func SetRoutes(app *iris.Application, users routes.Users) {
	app.Post("/users/:id", users.CreateUser)
	app.Get("/users/:id", users.GetUser)
	app.Put("/users/:id/:email", users.UpdateUser)
	app.Delete("/users/:id", users.DeleteUser)
	// app.Get("/users/confirmations/:token", users.Confirm)
	// app.Get("/files?startDate=<timestamp>?tag=<tag>", files.ListFiles)
	// app.Get("/file-ids/:name", files.GetFileId)
	// app.Get("/files/:file?skip=<number>&limit=<number>&exclude=<node-ids>", files.GetPointers)
	// app.Delete("/files/:file", files.DeleteFile)
	// app.Post("/files", files.NewFile)
	// app.Put("/files/:file/shards/:index", files.AddShardToFile)
	// app.Post("/reports", reports.CreateReport)
	// app.Get("/contacts?address=<address>&skip=<number>&limit=<number>", contacts.GetContacts)

}

func CacheRoutes(app *iris.Application, cache routes.Cache) {
	app.Post("/cache/:node/:addr", cache.Set)
	app.Get("/cache/:node", cache.GetNodeAddress)
}
