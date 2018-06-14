// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"google.golang.org/grpc"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/process"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

func TestNewServer(t *testing.T) {
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)

	srv := NewServer(nil, nil, nil, nil)
	assert.NotNil(t, srv)

	go srv.Serve(lis)
	srv.Stop()
}

func TestNewClient(t *testing.T) {
	//a := "35.232.202.229:8080"
	//c, err := NewClient(&a, grpc.WithInsecure())
	t.SkipNow()
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
	assert.NoError(t, err)
	srv := NewServer(nil, nil, nil, nil)
	go srv.Serve(lis)
	defer srv.Stop()

	address := lis.Addr().String()
	c, err := NewClient(&address, grpc.WithInsecure())
	assert.NoError(t, err)

	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
	assert.NoError(t, err)
	assert.NotNil(t, r)
}

func TestProcess_redis(t *testing.T) {
	flag.Set("localPort", "8889")
	done := test.EnsureRedis(t)
	defer done()

	o := Service{}
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	err := o.Process(ctx)
	assert.NoError(t, err)
}

func TestProcess_bolt(t *testing.T) {
	flag.Set("localPort", "8890")
	boltdbPath, err := filepath.Abs("test_bolt.db")
	assert.NoError(t, err)

	if err != nil {
		defer func() {
			if err := os.Remove(boltdbPath); err != nil {
				log.Printf("%s\n", errs.New("error while removing test bolt db: %s", err))
			}
		}()
	}

	flag.Set("boltdbPath", boltdbPath)

	o := Service{}
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	err = o.Process(ctx)
	assert.NoError(t, err)
}

func TestProcess_error(t *testing.T) {
	flag.Set("localPort", "8891")
	flag.Set("boltdbPath", "")
	flag.Set("redisAddress", "")

	o := Service{}
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	err := o.Process(ctx)
	assert.True(t, process.ErrUsage.Has(err))
}
