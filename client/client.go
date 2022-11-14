package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"

	pb "github.com/Lukski175/Assignment4/time"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:            ownPort,
		criticalQueue: make([]int32, 1),
		clients:       make(map[int32]pb.CriticalClient),
		ctx:           ctx,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterCriticalServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < 3; i++ {
		port := int32(5000) + int32(i)

		if port == ownPort {
			continue
		}

		var conn *grpc.ClientConn
		fmt.Printf("Trying to dial: %v\n", port)
		conn, err := grpc.Dial(fmt.Sprintf(":%v", port), grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("Could not connect: %s", err)
		}
		defer conn.Close()
		c := pb.NewCriticalClient(conn)
		p.clients[port] = c
	}

	//Request access
	for _, cl := range p.clients {
		cl.RequestAccess(ctx, &pb.Peer{Id: p.id, Time: timestamppb.Now()})
	}

	for {

	}
}

type peer struct {
	pb.UnimplementedCriticalServer
	id            int32
	criticalQueue []int32
	clients       map[int32]pb.CriticalClient
	ctx           context.Context
}

func (p *peer) RequestAccess(ctx context.Context, peer *pb.Peer) (pb.Reply, error) {
	p.criticalQueue = append(p.criticalQueue, peer.Id)
	log.Printf("Received request to enter critical zone from peer: %d, at time %s", peer.Id, peer.Time)
	return pb.Reply{}, nil
}
