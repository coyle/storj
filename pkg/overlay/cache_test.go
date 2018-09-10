// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/test"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/storelogger"
)

var (
	ctx = context.Background()
)

type dbClient int
type responses map[dbClient]*overlay.Node
type responsesB map[dbClient][]*overlay.Node
type errors map[dbClient]*errs.Class

const (
	mock dbClient = iota
	bolt
	_redis
	testNetSize = 30
)

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT, b overlay.Node) *kademlia.Kademlia {
	i, err := kademlia.NewID()
	assert.NoError(t, err)
	id := *i
	n := []overlay.Node{b}
	kad, err := kademlia.NewKademlia(&id, n, ip, port)
	assert.NoError(t, err)

	return kad
}

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, overlay.Node) {
	bid, err := kademlia.NewID()
	assert.NoError(t, err)

	bnid := *bid
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p)
	assert.NoError(t, err)
	intro, err := kademlia.GetIntroNode(bnid.String(), ip, pm)
	assert.NoError(t, err)

	boot, err := kademlia.NewKademlia(&bnid, []overlay.Node{*intro}, ip, pm)

	assert.NoError(t, err)
	rt, err := boot.GetRoutingTable(context.Background())
	assert.NoError(t, err)
	bootNode := rt.Local()

	err = boot.ListenAndServe()
	assert.NoError(t, err)
	p++

	err = boot.Bootstrap(context.Background())
	assert.NoError(t, err)
	for i := 0; i < testNetSize; i++ {
		gg := strconv.Itoa(p)

		nid, err := kademlia.NewID()
		assert.NoError(t, err)
		id := *nid

		dht, err := kademlia.NewKademlia(&id, []overlay.Node{bootNode}, ip, gg)
		assert.NoError(t, err)

		p++
		dhts = append(dhts, dht)
		err = dht.ListenAndServe()
		assert.NoError(t, err)
		err = dht.Bootstrap(context.Background())
		assert.NoError(t, err)
	}

	return dhts, bootNode
}

var (
	getCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		expectedResponses   responses
		expectedErrors      errors
		data                test.KvStore
	}{
		{
			testID:              "valid Get",
			expectedTimesCalled: 1,
			key:                 "foo",
			expectedResponses: func() responses {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				return responses{
					mock:   na,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{"foo": func() storage.Value {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
		{
			testID:              "forced get error",
			expectedTimesCalled: 1,
			key:                 "error",
			expectedResponses: func() responses {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				return responses{
					mock:   nil,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{"error": func() storage.Value {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
		{
			testID:              "get missing key",
			expectedTimesCalled: 1,
			key:                 "bar",
			expectedResponses: responses{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			// TODO(bryanchriswhite): compare actual errors
			expectedErrors: errors{
				mock:   nil,
				bolt:   &storage.ErrKeyNotFound,
				_redis: &storage.ErrKeyNotFound,
			},
			data: test.KvStore{"foo": func() storage.Value {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
	}
	getAllCases = []struct {
		testID              string
		expectedTimesCalled int
		keys                []string
		expectedResponses   responsesB
		expectedErrors      errors
		data                test.KvStore
	}{
		{testID: "valid GetAll",
			expectedTimesCalled: 1,
			keys:                []string{"key1"},
			expectedResponses: func() responsesB {
				n1 := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				ns := []*overlay.Node{n1}
				return responsesB{
					mock:   ns,
					bolt:   ns,
					_redis: ns,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{
				"key1": func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			},
		},
		{testID: "valid GetAll",
			expectedTimesCalled: 1,
			keys:                []string{"key1", "key2"},
			expectedResponses: func() responsesB {
				n1 := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				n2 := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9998"}}
				ns := []*overlay.Node{n1, n2}
				return responsesB{
					mock:   ns,
					bolt:   ns,
					_redis: ns,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{
				"key1": func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
				"key2": func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9998"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			},
		},
		{testID: "mix of valid and nil nodes returned",
			expectedTimesCalled: 1,
			keys:                []string{"key1", "key3"},
			expectedResponses: func() responsesB {
				n1 := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				ns := []*overlay.Node{n1, nil}
				return responsesB{
					mock:   ns,
					bolt:   ns,
					_redis: ns,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{
				"key1": func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			},
		},
		{testID: "empty string keys",
			expectedTimesCalled: 1,
			keys:                []string{"", ""},
			expectedResponses: func() responsesB {
				ns := []*overlay.Node{nil, nil}
				return responsesB{
					mock:   ns,
					bolt:   ns,
					_redis: ns,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
		},
		{testID: "empty keys",
			expectedTimesCalled: 0,
			keys:                []string{},
			expectedResponses: func() responsesB {
				return responsesB{
					mock:   nil,
					bolt:   nil,
					_redis: nil,
				}
			}(),
			expectedErrors: errors{
				mock:   &OverlayError,
				bolt:   &OverlayError,
				_redis: &OverlayError,
			},
		},
	}
	putCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		value               overlay.Node
		expectedErrors      errors
		data                test.KvStore
	}{
		{
			testID:              "valid Put",
			expectedTimesCalled: 1,
			key:                 "foo",
			value:               overlay.Node{Id: "foo", Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}},
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: test.KvStore{},
		},
	}

	refreshCases = []struct {
		testID              string
		expectedTimesCalled int
		expectedErr         error
		data                test.KvStore
	}{
		{
			testID:              "valid update",
			expectedTimesCalled: 1,
			expectedErr:         nil,
			data:                test.KvStore{},
		},
	}
)

func redisTestClient(t *testing.T, addr string, data test.KvStore) storage.KeyValueStore {
	client, err := redis.NewClient(addr, "", 1)
	if err != nil {
		t.Fatal(err)
	}

	if !(data.Empty()) {
		populateStorage(t, client, data)
	}

	return client
}

func boltTestClient(t *testing.T, data test.KvStore) (_ storage.KeyValueStore, _ func()) {
	boltPath, err := filepath.Abs("test_bolt.db")
	assert.NoError(t, err)

	client, err := boltdb.New(boltPath, "testBoltdb")
	assert.NoError(t, err)

	cleanup := func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, os.Remove(boltPath))
	}

	if !(data.Empty()) {
		populateStorage(t, client, data)
	}

	return storelogger.New(zaptest.NewLogger(t), client), cleanup
}

func populateStorage(t *testing.T, client storage.KeyValueStore, data test.KvStore) {
	for k, v := range data {
		err := client.Put(storage.Key(k), v)
		assert.NoError(t, err)
	}
}

func TestRedisGet(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(t, redisAddr, c.data)
			oc := Cache{DB: db}

			resp, err := oc.Get(ctx, c.key)
			assertErrClass(t, c.expectedErrors[_redis], err)
			assert.Equal(t, c.expectedResponses[_redis], resp)
		})
	}
}

func TestRedisGetAll(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	for _, c := range getAllCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(t, c.data)
			oc := Cache{DB: db}

			resp, err := oc.GetAll(ctx, c.keys)
			assertErrClass(t, c.expectedErrors[_redis], err)
			assert.Equal(t, c.expectedResponses[_redis], resp)
		})
	}
}

func assertErrClass(t *testing.T, class *errs.Class, err error) {
	if class != nil {
		assert.True(t, class.Has(err))
	} else {
		assert.NoError(t, err)
	}
}

func TestRedisPut(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(t, redisAddr, c.data)
			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)

			na := &overlay.Node{}
			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestBoltGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(t, c.data)
			defer cleanup()

			oc := Cache{DB: db}

			resp, err := oc.Get(ctx, c.key)
			assertErrClass(t, c.expectedErrors[bolt], err)
			assert.Equal(t, c.expectedResponses[bolt], resp)
		})
	}
}

func TestBoltGetAll(t *testing.T) {
	for _, c := range getAllCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(t, c.data)
			defer cleanup()
			oc := Cache{DB: db}
			resp, err := oc.GetAll(ctx, c.keys)
			assertErrClass(t, c.expectedErrors[bolt], err)
			assert.Equal(t, c.expectedResponses[bolt], resp)
		})
	}
}
func TestBoltPut(t *testing.T) {
	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(t, c.data)
			defer cleanup()

			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)
			na := &overlay.Node{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestMockGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {

			db := test.NewMockKeyValueStore(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.GetCalled)

			resp, err := oc.Get(ctx, c.key)
			assertErrClass(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedResponses[mock], resp)
			assert.Equal(t, c.expectedTimesCalled, db.GetCalled)
		})
	}
}

func TestMockGetAll(t *testing.T) {
	for _, c := range getAllCases {
		t.Run(c.testID, func(t *testing.T) {

			db := test.NewMockKeyValueStore(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.GetAllCalled)

			resp, err := oc.GetAll(ctx, c.keys)
			assertErrClass(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedResponses[mock], resp)
			assert.Equal(t, c.expectedTimesCalled, db.GetAllCalled)
		})
	}
}

func TestMockPut(t *testing.T) {
	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {

			db := test.NewMockKeyValueStore(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.PutCalled)

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedTimesCalled, db.PutCalled)

			v := db.Data[c.key]
			na := &overlay.Node{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestRefresh(t *testing.T) {
	t.Skip()
	for _, c := range refreshCases {
		t.Run(c.testID, func(t *testing.T) {
			dhts, b := bootstrapTestNetwork(t, "127.0.0.1", "3000")
			ctx := context.Background()
			db := test.NewMockKeyValueStore(c.data)
			dht := newTestKademlia(t, "127.0.0.1", "2999", dhts[rand.Intn(testNetSize)], b)

			_cache := &Cache{
				DB:  db,
				DHT: dht,
			}

			err := _cache.Refresh(ctx)
			assert.Equal(t, err, c.expectedErr)
		})
	}
}

func TestNewRedisOverlayCache(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	cases := []struct {
		testName, address string
		testFunc          func(string)
	}{
		{
			testName: "NewRedisOverlayCache valid",
			address:  redisAddr,
			testFunc: func(address string) {
				cache, err := NewRedisOverlayCache(address, "", 1, nil)

				assert.NoError(t, err)
				assert.NotNil(t, cache)
			},
		},
		{
			testName: "NewRedisOverlayCache fail",
			address:  "",
			testFunc: func(address string) {
				cache, err := NewRedisOverlayCache(address, "", 1, nil)

				assert.Error(t, err)
				assert.Nil(t, cache)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc(c.address)
		})
	}
}
