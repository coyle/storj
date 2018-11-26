// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/internal/storj"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/teststore"
)

// newTestRoutingTable returns a newly configured instance of a RoutingTable
func newTestRoutingTable(localNode pb.Node) (*RoutingTable, error) {
	rt := &RoutingTable{
		self:         localNode,
		kadBucketDB:  storelogger.New(zap.L(), teststore.New()),
		nodeBucketDB: storelogger.New(zap.L(), teststore.New()),
		transport:    &defaultTransport,

		mutex:            &sync.Mutex{},
		seen:             make(map[storj.NodeID]*pb.Node),
		replacementCache: make(map[bucketID][]*pb.Node),

		idLength:     16,
		bucketSize:   6,
		rcBucketSize: 2,
	}
	ok, err := rt.addNode(&localNode)
	if !ok || err != nil {
		return nil, RoutingErr.New("could not add localNode to routing table: %s", err)
	}
	return rt, nil
}

func createRoutingTable(t *testing.T, localNodeID storj.NodeID) (*RoutingTable, func()) {
	if localNodeID == (storj.NodeID{}) {
		localNodeID = teststorj.NodeIDFromString("AA")
	}
	localNode := pb.Node{Id: localNodeID}

	rt, err := newTestRoutingTable(localNode)
	if err != nil {
		t.Fatal(err)
	}

	return rt, func() {
		err := rt.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func mockNode(s string) *pb.Node {
	id := teststorj.NodeIDFromString(s)
	var node pb.Node
	node.Id = id
	return &node
}

func TestAddNode(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromString("OO"))
	defer cleanup()
	// bucket, err := rt.kadBucketDB.Get(storage.Key([]byte{255, 255}))
	// assert.NoError(t, err)
	// assert.NotNil(t, bucket)
	cases := []struct {
		testID  string
		node    *pb.Node
		added   bool
		kadIDs  [][]byte
		nodeIDs [][]string
	}{
		{testID: "PO: add node to unfilled kbucket",
			node:    mockNode("PO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"OO", "PO"}},
		},
		{testID: "NO: add node to full kbucket and split",
			node:    mockNode("NO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"NO", "OO", "PO"}},
		},
		{testID: "MO",
			node:    mockNode("MO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"MO", "NO", "OO", "PO"}},
		},
		{testID: "LO",
			node:    mockNode("LO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"LO", "MO", "NO", "OO", "PO"}},
		},
		{testID: "QO",
			node:    mockNode("QO"),
			added:   true,
			kadIDs:  [][]byte{{255, 255}},
			nodeIDs: [][]string{{"LO", "MO", "NO", "OO", "PO", "QO"}},
		},
		{testID: "SO: split bucket",
			node:    mockNode("SO"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "?O",
			node:    mockNode("?O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ">O",
			node:   mockNode(">O"),
			added:  true,
			kadIDs: [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}}, nodeIDs: [][]string{{">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "=O",
			node:    mockNode("=O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ";O",
			node:    mockNode(";O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: ":O",
			node:    mockNode(":O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "9O",
			node:    mockNode("9O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "8O: should drop",
			node:    mockNode("8O"),
			added:   false,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "KO",
			node:    mockNode("KO"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "JO",
			node:    mockNode("JO"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO"}, {}, {}},
		},
		{testID: "]O",
			node:    mockNode("]O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O"}, {}, {}},
		},
		{testID: "^O",
			node:    mockNode("^O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O"}, {}, {}},
		},
		{testID: "_O",
			node:    mockNode("_O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O", "_O"}, {}, {}},
		},
		{testID: "@O: split bucket 2",
			node:    mockNode("@O"),
			added:   true,
			kadIDs:  [][]byte{{63, 255}, {71, 255}, {79, 255}, {95, 255}, {127, 255}, {255, 255}},
			nodeIDs: [][]string{{"9O", ":O", ";O", "=O", ">O", "?O"}, {"@O"}, {"JO", "KO", "LO", "MO", "NO", "OO"}, {"PO", "QO", "SO", "]O", "^O", "_O"}, {}, {}},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ok, err := rt.addNode(c.node)
			assert.Equal(t, c.added, ok)
			assert.NoError(t, err)
			kadKeys, err := rt.kadBucketDB.List(nil, 0)
			assert.NoError(t, err)
			for i, v := range kadKeys {
				assert.True(t, bytes.Equal(c.kadIDs[i], v[:2]) == true)
				ids, err := rt.getNodeIDsWithinKBucket(keyToBucketID(v))
				assert.NoError(t, err)
				fmt.Printf("TOTAL=%d\n", len(ids))
				for j, id := range ids {
					fmt.Printf("[%v][%d]==[%v]\n", c.nodeIDs[i], j, id.String())
					// assert.True(t, teststorj.NodeIDFromString(c.nodeIDs[i][j]), id.Bytes())
				}
			}

			if c.testID == "8O" {
				nodeID80 := teststorj.NodeIDFromString("8O")
				n := rt.replacementCache[keyToBucketID(nodeID80.Bytes())]
				assert.Equal(t, nodeID80.Bytes(), n[0].Id.Bytes())
			}

		})
	}
}

func TestUpdateNode(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromString("AA"))
	defer cleanup()
	node := mockNode("BB")
	ok, err := rt.addNode(node)
	assert.True(t, ok)
	assert.NoError(t, err)
	val, err := rt.nodeBucketDB.Get(node.Id.Bytes())
	assert.NoError(t, err)
	unmarshaled, err := unmarshalNodes([]storage.Value{val})
	assert.NoError(t, err)
	x := unmarshaled[0].Address
	assert.Nil(t, x)

	node.Address = &pb.NodeAddress{Address: "BB"}
	err = rt.updateNode(node)
	assert.NoError(t, err)
	val, err = rt.nodeBucketDB.Get(node.Id.Bytes())
	assert.NoError(t, err)
	unmarshaled, err = unmarshalNodes([]storage.Value{val})
	assert.NoError(t, err)
	y := unmarshaled[0].Address.Address
	assert.Equal(t, "BB", y)
}

func TestRemoveNode(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromString("AA"))
	defer cleanup()
	kadBucketID := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	node := mockNode("BB")
	ok, err := rt.addNode(node)
	assert.True(t, ok)
	assert.NoError(t, err)
	val, err := rt.nodeBucketDB.Get(node.Id.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, val)
	node2 := mockNode("CC")
	rt.addToReplacementCache(kadBucketID, node2)
	err = rt.removeNode(node.Id)
	assert.NoError(t, err)
	val, err = rt.nodeBucketDB.Get(node.Id.Bytes())
	assert.Nil(t, val)
	assert.Error(t, err)
	val2, err := rt.nodeBucketDB.Get(node2.Id.Bytes())
	assert.NoError(t, err)
	assert.NotNil(t, val2)
	assert.Equal(t, 0, len(rt.replacementCache[kadBucketID]))

	//try to remove node not in rt
	err = rt.removeNode(teststorj.NodeIDFromString("DD"))
	assert.NoError(t, err)
}

func TestCreateOrUpdateKBucket(t *testing.T) {
	id := teststorj.NodeIDFromBytes([]byte{255, 255})
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	err := rt.createOrUpdateKBucket(keyToBucketID(id.Bytes()), time.Now())
	assert.NoError(t, err)
	val, e := rt.kadBucketDB.Get(id.Bytes())
	assert.NotNil(t, val)
	assert.NoError(t, e)

}

func TestGetKBucketID(t *testing.T) {
	kadIDA := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	nodeIDA := teststorj.NodeIDFromString("AA")
	rt, cleanup := createRoutingTable(t, nodeIDA)
	defer cleanup()
	keyA, err := rt.getKBucketID(nodeIDA)
	assert.NoError(t, err)
	assert.Equal(t, kadIDA, keyA)
}

func TestXorTwoIds(t *testing.T) {
	x := xorTwoIds([]byte{191}, []byte{159})
	assert.Equal(t, []byte{32}, x) //00100000
}

func TestSortByXOR(t *testing.T) {
	node1 := teststorj.NodeIDFromBytes([]byte{127, 255}) //xor 0
	rt, cleanup := createRoutingTable(t, node1)
	defer cleanup()
	node2 := teststorj.NodeIDFromBytes([]byte{143, 255}) //xor 240
	assert.NoError(t, rt.nodeBucketDB.Put(node2.Bytes(), []byte("")))
	node3 := teststorj.NodeIDFromBytes([]byte{255, 255}) //xor 128
	assert.NoError(t, rt.nodeBucketDB.Put(node3.Bytes(), []byte("")))
	node4 := teststorj.NodeIDFromBytes([]byte{191, 255}) //xor 192
	assert.NoError(t, rt.nodeBucketDB.Put(node4.Bytes(), []byte("")))
	node5 := teststorj.NodeIDFromBytes([]byte{133, 255}) //xor 250
	assert.NoError(t, rt.nodeBucketDB.Put(node5.Bytes(), []byte("")))
	nodes, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	expectedNodes := storage.Keys{node1.Bytes(), node5.Bytes(), node2.Bytes(), node4.Bytes(), node3.Bytes()}
	assert.Equal(t, expectedNodes, nodes)
	sortByXOR(nodes, node1.Bytes())
	expectedSorted := storage.Keys{node1.Bytes(), node3.Bytes(), node4.Bytes(), node2.Bytes(), node5.Bytes()}
	assert.Equal(t, expectedSorted, nodes)
	nodes, err = rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, expectedNodes, nodes)
}

func BenchmarkSortByXOR(b *testing.B) {
	nodes := []storage.Key{}

	newNodeID := func() storage.Key {
		id := make(storage.Key, 32)
		rand.Read(id[:])
		return id
	}

	for k := 0; k < 1000; k++ {
		nodes = append(nodes, newNodeID())
	}

	b.ResetTimer()
	for m := 0; m < b.N; m++ {
		rand.Shuffle(len(nodes), func(i, k int) {
			nodes[i], nodes[k] = nodes[k], nodes[i]
		})

		sortByXOR(nodes, newNodeID())
	}
}

func TestDetermineFurthestIDWithinK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{127, 255}))
	defer cleanup()
	cases := []struct {
		testID           string
		nodeID           []byte
		expectedFurthest []byte
	}{
		{testID: "xor 0",
			nodeID:           []byte{127, 255},
			expectedFurthest: []byte{127, 255},
		},
		{testID: "xor 240",
			nodeID:           []byte{143, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 128",
			nodeID:           []byte{255, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 192",
			nodeID:           []byte{191, 255},
			expectedFurthest: []byte{143, 255},
		},
		{testID: "xor 250",
			nodeID:           []byte{133, 255},
			expectedFurthest: []byte{133, 255},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			assert.NoError(t, rt.nodeBucketDB.Put(c.nodeID, []byte("")))
			nodes, err := rt.nodeBucketDB.List(nil, 0)
			assert.NoError(t, err)
			furthest, err := rt.determineFurthestIDWithinK(teststorj.NodeIDsFromBytes(nodes.ByteSlices()...))
			assert.NoError(t, err)
			assert.Equal(t, c.expectedFurthest, furthest)
		})
	}
}

func TestNodeIsWithinNearestK(t *testing.T) {
	rt, cleanup := createRoutingTable(t, teststorj.NodeIDFromBytes([]byte{127, 255}))
	defer cleanup()
	rt.bucketSize = 2
	cases := []struct {
		testID  string
		nodeID  []byte
		closest bool
	}{
		{testID: "A",
			nodeID:  []byte{127, 255},
			closest: true,
		},
		{testID: "B",
			nodeID:  []byte{143, 255},
			closest: true,
		},
		{testID: "C",
			nodeID:  []byte{255, 255},
			closest: true,
		},
		{testID: "D",
			nodeID:  []byte{191, 255},
			closest: true,
		},
		{testID: "E",
			nodeID:  []byte{133, 255},
			closest: false,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			result, err := rt.nodeIsWithinNearestK(teststorj.NodeIDFromBytes(c.nodeID))
			assert.NoError(t, err)
			assert.Equal(t, c.closest, result)
			assert.NoError(t, rt.nodeBucketDB.Put(c.nodeID, []byte("")))
		})
	}
}

func TestKadBucketContainsLocalNode(t *testing.T) {
	nodeIDA := teststorj.NodeIDFromBytes([]byte{183, 255}) //[10110111, 1111111]
	rt, cleanup := createRoutingTable(t, nodeIDA)
	defer cleanup()
	kadIDA := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	kadIDB := keyToBucketID(teststorj.NodeIDFromBytes([]byte{127, 255}).Bytes())
	now := time.Now()
	err := rt.createOrUpdateKBucket(kadIDB, now)
	assert.NoError(t, err)
	resultTrue, err := rt.kadBucketContainsLocalNode(kadIDA)
	assert.NoError(t, err)
	resultFalse, err := rt.kadBucketContainsLocalNode(kadIDB)
	assert.NoError(t, err)
	assert.True(t, resultTrue)
	assert.False(t, resultFalse)
}

func TestKadBucketHasRoom(t *testing.T) {
	node1 := teststorj.NodeIDFromBytes([]byte{255, 255})
	kadIDA := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	rt, cleanup := createRoutingTable(t, node1)
	defer cleanup()
	node2 := []byte{191, 255}
	node3 := []byte{127, 255}
	node4 := []byte{63, 255}
	node5 := []byte{159, 255}
	node6 := []byte{0, 127}
	resultA, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.True(t, resultA)
	assert.NoError(t, rt.nodeBucketDB.Put(node2, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node3, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node4, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node5, []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(node6, []byte("")))
	resultB, err := rt.kadBucketHasRoom(kadIDA)
	assert.NoError(t, err)
	assert.False(t, resultB)
}

func TestGetNodeIDsWithinKBucket(t *testing.T) {
	nodeIDA := teststorj.NodeIDFromBytes([]byte{183, 255}) //[10110111, 1111111]
	rt, cleanup := createRoutingTable(t, nodeIDA)
	defer cleanup()
	kadIDA := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	kadIDB := keyToBucketID(teststorj.NodeIDFromBytes([]byte{127, 255}).Bytes())
	now := time.Now()
	assert.NoError(t, rt.createOrUpdateKBucket(kadIDB, now))

	nodeIDB := teststorj.NodeIDFromBytes([]byte{111, 255}) //[01101111, 1111111]
	nodeIDC := teststorj.NodeIDFromBytes([]byte{47, 255})  //[00101111, 1111111]

	assert.NoError(t, rt.nodeBucketDB.Put(nodeIDB.Bytes(), []byte("")))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeIDC.Bytes(), []byte("")))

	cases := []struct {
		testID   string
		kadID    bucketID
		expected storage.Keys
	}{
		{testID: "A",
			kadID:    kadIDA,
			expected: storage.Keys{nodeIDA.Bytes()},
		},
		{testID: "B",
			kadID:    kadIDB,
			expected: storage.Keys{nodeIDC.Bytes(), nodeIDB.Bytes()},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			n, err := rt.getNodeIDsWithinKBucket(c.kadID)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, n)
		})
	}
}

func TestGetNodesFromIDs(t *testing.T) {
	nodeA := mockNode("AA")
	nodeB := mockNode("BB")
	nodeC := mockNode("CC")
	a, err := proto.Marshal(nodeA)
	assert.NoError(t, err)
	b, err := proto.Marshal(nodeB)
	assert.NoError(t, err)
	c, err := proto.Marshal(nodeC)
	assert.NoError(t, err)
	rt, cleanup := createRoutingTable(t, nodeA.Id)
	defer cleanup()

	assert.NoError(t, rt.nodeBucketDB.Put(nodeA.Id.Bytes(), a))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeB.Id.Bytes(), b))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeC.Id.Bytes(), c))
	expected := []*pb.Node{nodeA, nodeB, nodeC}

	nodeKeys, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	values, err := rt.getNodesFromIDsBytes(teststorj.NodeIDsFromBytes(nodeKeys.ByteSlices()...))
	assert.NoError(t, err)
	assert.Equal(t, expected, values)
}

func TestUnmarshalNodes(t *testing.T) {
	nodeA := mockNode("AA")
	nodeB := mockNode("BB")
	nodeC := mockNode("CC")

	a, err := proto.Marshal(nodeA)
	assert.NoError(t, err)
	b, err := proto.Marshal(nodeB)
	assert.NoError(t, err)
	c, err := proto.Marshal(nodeC)
	assert.NoError(t, err)
	rt, cleanup := createRoutingTable(t, nodeA.Id)
	defer cleanup()
	assert.NoError(t, rt.nodeBucketDB.Put(nodeA.Id.Bytes(), a))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeB.Id.Bytes(), b))
	assert.NoError(t, rt.nodeBucketDB.Put(nodeC.Id.Bytes(), c))
	nodeKeys, err := rt.nodeBucketDB.List(nil, 0)
	assert.NoError(t, err)
	nodes, err := rt.getNodesFromIDsBytes(teststorj.NodeIDsFromBytes(nodeKeys.ByteSlices()...))
	assert.NoError(t, err)
	expected := []*pb.Node{nodeA, nodeB, nodeC}
	for i, v := range expected {
		assert.True(t, proto.Equal(v, nodes[i]))
	}
}

func TestGetUnmarshaledNodesFromBucket(t *testing.T) {
	bucketID := keyToBucketID(teststorj.NodeIDFromBytes([]byte{255, 255}).Bytes())
	nodeA := mockNode("AA")
	rt, cleanup := createRoutingTable(t, nodeA.Id)
	defer cleanup()
	nodeB := mockNode("BB")
	nodeC := mockNode("CC")
	var err error
	_, err = rt.addNode(nodeB)
	assert.NoError(t, err)
	_, err = rt.addNode(nodeC)
	assert.NoError(t, err)
	nodes, err := rt.getUnmarshaledNodesFromBucket(bucketID)
	expected := []*pb.Node{nodeA, nodeB, nodeC}
	assert.NoError(t, err)
	for i, v := range expected {
		assert.True(t, proto.Equal(v, nodes[i]))
	}
}

func TestGetKBucketRange(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	idA := teststorj.NodeIDFromBytes([]byte{255, 255})
	idB := teststorj.NodeIDFromBytes([]byte{127, 255})
	idC := teststorj.NodeIDFromBytes([]byte{63, 255})
	assert.NoError(t, rt.kadBucketDB.Put(idA.Bytes(), []byte("")))
	assert.NoError(t, rt.kadBucketDB.Put(idB.Bytes(), []byte("")))
	assert.NoError(t, rt.kadBucketDB.Put(idC.Bytes(), []byte("")))
	zeroBID := bucketID{}
	cases := []struct {
		testID   string
		id       storj.NodeID
		expected storage.Keys
	}{
		{testID: "A",
			id:       idA,
			expected: storage.Keys{idB.Bytes(), idA.Bytes()},
		},
		{testID: "B",
			id:       idB,
			expected: storage.Keys{idC.Bytes(), idB.Bytes()}},
		{testID: "C",
			id:       idC,
			expected: storage.Keys{zeroBID[:], idC.Bytes()},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			ep, err := rt.getKBucketRange(keyToBucketID(c.id.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, c.expected, ep)
		})
	}
}

func TestCreateFirstBucketID(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	x := rt.createFirstBucketID()
	expected := teststorj.NodeIDFromBytes([]byte{255, 255})
	assert.Equal(t, x, expected.Bytes())
}

func TestBucketIDZeroValue(t *testing.T) {
	// rt, cleanup := createRoutingTable(t, storj.NodeID{})
	// defer cleanup()
	zero := bucketID{} //rt.createZeroAsBucketID()
	expected := teststorj.NodeIDFromBytes([]byte{0, 0})
	assert.Equal(t, zero, storage.Key(expected.Bytes()))
}

func TestDetermineLeafDepth(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	idA := teststorj.NodeIDFromBytes([]byte{255, 255})
	idB := teststorj.NodeIDFromBytes([]byte{127, 255})
	idC := teststorj.NodeIDFromBytes([]byte{63, 255})

	cases := []struct {
		testID  string
		id      storj.NodeID
		depth   int
		addNode func()
	}{
		{testID: "A",
			id:    idA,
			depth: 0,
			addNode: func() {
				e := rt.kadBucketDB.Put(idA.Bytes(), []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "B",
			id:    idB,
			depth: 1,
			addNode: func() {
				e := rt.kadBucketDB.Put(idB.Bytes(), []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "C",
			id:    idA,
			depth: 1,
			addNode: func() {
				e := rt.kadBucketDB.Put(idC.Bytes(), []byte(""))
				assert.NoError(t, e)
			},
		},
		{testID: "D",
			id:      idB,
			depth:   2,
			addNode: func() {},
		},
		{testID: "E",
			id:      idC,
			depth:   2,
			addNode: func() {},
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			c.addNode()
			d, err := rt.determineLeafDepth(keyToBucketID(c.id.Bytes()))
			assert.NoError(t, err)
			assert.Equal(t, c.depth, d)
		})
	}
}

func TestDetermineDifferingBitIndex(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	cases := []struct {
		testID   string
		bucketID bucketID
		key      bucketID
		expected int
		err      *errs.Class
	}{
		{testID: "A",
			bucketID: keyToBucketID([]byte{191, 255}),
			key:      keyToBucketID([]byte{255, 255}),
			expected: 1,
			err:      nil,
		},
		{testID: "B",
			bucketID: keyToBucketID([]byte{255, 255}),
			key:      keyToBucketID([]byte{191, 255}),
			expected: 1,
			err:      nil,
		},
		{testID: "C",
			bucketID: keyToBucketID([]byte{95, 255}),
			key:      keyToBucketID([]byte{127, 255}),
			expected: 2,
			err:      nil,
		},
		{testID: "D",
			bucketID: keyToBucketID([]byte{95, 255}),
			key:      keyToBucketID([]byte{79, 255}),
			expected: 3,
			err:      nil,
		},
		{testID: "E",
			bucketID: keyToBucketID([]byte{95, 255}),
			key:      keyToBucketID([]byte{63, 255}),
			expected: 2,
			err:      nil,
		},
		{testID: "F",
			bucketID: keyToBucketID([]byte{95, 255}),
			key:      keyToBucketID([]byte{79, 255}),
			expected: 3,
			err:      nil,
		},
		{testID: "G",
			bucketID: keyToBucketID([]byte{255, 255}),
			key:      keyToBucketID([]byte{255, 255}),
			expected: -2,
			err:      &RoutingErr,
		},
		{testID: "H",
			bucketID: keyToBucketID([]byte{255, 255}),
			key:      keyToBucketID([]byte{0, 0}),
			expected: -1,
			err:      nil,
		},
		{testID: "I",
			bucketID: keyToBucketID([]byte{127, 255}),
			key:      keyToBucketID([]byte{0, 0}),
			expected: 0,
			err:      nil,
		},
		{testID: "J",
			bucketID: keyToBucketID([]byte{63, 255}),
			key:      keyToBucketID([]byte{0, 0}),
			expected: 1,
			err:      nil,
		},
		{testID: "K",
			bucketID: keyToBucketID([]byte{31, 255}),
			key:      keyToBucketID([]byte{0, 0}),
			expected: 2,
			err:      nil,
		},
		{testID: "L",
			bucketID: keyToBucketID([]byte{95, 255}),
			key:      keyToBucketID([]byte{63, 255}),
			expected: 2,
			err:      nil,
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			diff, err := rt.determineDifferingBitIndex(c.bucketID, c.key)
			assertErrClass(t, c.err, err)
			assert.Equal(t, c.expected, diff)
		})
	}
}

func TestSplitBucket(t *testing.T) {
	rt, cleanup := createRoutingTable(t, storj.NodeID{})
	defer cleanup()
	cases := []struct {
		testID string
		idA    []byte
		idB    []byte
		depth  int
	}{
		{testID: "A: [11111111, 11111111] -> [10111111, 11111111]",
			idA:   []byte{255, 255},
			idB:   []byte{191, 255},
			depth: 1,
		},
		{testID: "B: [10111111, 11111111] -> [10011111, 11111111]",
			idA:   []byte{191, 255},
			idB:   []byte{159, 255},
			depth: 2,
		},
		{testID: "C: [01111111, 11111111] -> [00111111, 11111111]",
			idA:   []byte{127, 255},
			idB:   []byte{63, 255},
			depth: 1,
		},
		{testID: "D: [00000000, 11111111] -> [00000000, 01111111]",
			idA:   []byte{0, 255},
			idB:   []byte{0, 127},
			depth: 8,
		},
		{testID: "E: [01011111, 11111111] -> [01010111, 11111111]",
			idA:   []byte{95, 255},
			idB:   []byte{87, 255},
			depth: 4,
		},
		{testID: "F: [01011111, 11111111] -> [01001111, 11111111]",
			idA:   []byte{95, 255},
			idB:   []byte{79, 255},
			depth: 3,
		},
	}
	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			newID := rt.splitBucket(keyToBucketID(teststorj.NodeIDFromBytes(c.idA).Bytes()), c.depth)
			assert.Equal(t, c.idB, newID)
		})
	}
}

func assertErrClass(t *testing.T, class *errs.Class, err error) {
	t.Helper()
	if class != nil {
		assert.True(t, class.Has(err))
	} else {
		assert.NoError(t, err)
	}
}
