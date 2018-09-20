// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// Server implements the grpc Node Server
type Server struct {
	dht dht.DHT
}

// NewServer returns a newly instantiated Node Server
func NewServer(dht dht.DHT) *Server {
	return &Server{
		dht: dht,
	}
}

// Query is a node to node communication query
func (s *Server) Query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	rt, err := s.dht.GetRoutingTable(ctx)
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not get routing table %s", err)
	}

	if req.GetPingback() {
		_, err = s.dht.Ping(ctx, *req.Sender)
		if err != nil {
			err = rt.ConnectionFailed(req.Sender)
			if err != nil {
				return &pb.QueryResponse{}, NodeClientErr.New("could not respond to connection failed %s", err)
			}
			return &pb.QueryResponse{}, NodeClientErr.New("connection to node %s failed", req.Sender.Id)
		}
		err = rt.ConnectionSuccess(req.Sender)
		if err != nil {
			return &pb.QueryResponse{}, NodeClientErr.New("could not respond to connection success %s", err)
		}
	}

	id := StringToID(req.Target.Id)
	nodes, err := rt.FindNear(id, int(req.Limit))
	if err != nil {
		return &pb.QueryResponse{}, NodeClientErr.New("could not find near %s", err)
	}

	return &pb.QueryResponse{Sender: req.Sender, Response: nodes}, nil
}
