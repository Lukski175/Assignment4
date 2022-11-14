package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	pb "github.com/Lukski175/Assignment4/time"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var numberOfClients int = 3
var criticalUnlocked bool = true

func main() {
	arg1, _ := strconv.ParseInt(os.Args[1], 10, 32)
	ownPort := int32(arg1) + 5000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := &peer{
		id:            ownPort,
		criticalQueue: make([]*pb.Peer, 0),
		clients:       make(map[int32]pb.CriticalClient),
		ctx:           ctx,
	}

	// Create listener tcp on port ownPort
	list, err := net.Listen("tcp", fmt.Sprintf(":%v", ownPort))
	if err != nil {
		log.Fatalf("Failed to listen on port: %v", err)
	}
	log.Printf("Setup client with id: %d", p.id)
	grpcServer := grpc.NewServer()
	pb.RegisterCriticalServer(grpcServer, p)

	go func() {
		if err := grpcServer.Serve(list); err != nil {
			log.Fatalf("failed to server %v", err)
		}
	}()

	for i := 0; i < numberOfClients; i++ {
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

	time.Sleep(1 * time.Second)

	//Propose candidate
	var lowestTime *timestamppb.Timestamp
	var candidate *pb.Peer
	for i := 0; i < len(p.criticalQueue); i++ {
		if i == 0 {
			candidate = p.criticalQueue[i]
			lowestTime = candidate.Time
		} else if lowestTime.AsTime().After(p.criticalQueue[i].Time.AsTime()) {
			candidate = p.criticalQueue[i]
			lowestTime = candidate.Time
		}
	}
	for _, cl := range p.clients {
		cl.ProposeCandidate(ctx, &pb.Peer{Id: candidate.Id, Time: lowestTime})
	}

	//Done with critical stuff
	time.Sleep(5 * time.Second)
	Critical(&pb.Peer{Id: p.id})
	for _, cl := range p.clients {
		cl.CriticalDone(ctx, &pb.Peer{Id: p.id})
	}

	//Loop
	for {

	}
}

type peer struct {
	pb.UnimplementedCriticalServer
	id            int32
	criticalQueue []*pb.Peer
	clients       map[int32]pb.CriticalClient
	ctx           context.Context
}

func (p *peer) RequestAccess(ctx context.Context, peer *pb.Peer) (*pb.Reply, error) {
	p.criticalQueue = append(p.criticalQueue, peer)
	log.Printf("Received request to enter critical zone from peer: %d, at time %s", peer.Id, peer.Time.AsTime().String())
	return &pb.Reply{}, nil
}

var proposedAmount int
var savingCandidate sync.Mutex
var candidates = make([]*pb.Peer, 0)
var finalCandidateId int

func (p *peer) ProposeCandidate(ctx context.Context, peer *pb.Peer) (*pb.Reply, error) {
	savingCandidate.Lock()
	proposedAmount++
	candidates = append(candidates, peer)
	if proposedAmount >= numberOfClients-1 {
		proposedAmount = 0
		var time *timestamppb.Timestamp
		var candidate *pb.Peer
		for i := 0; i < len(candidates); i++ {
			if i == 0 {
				candidate = candidates[i]
				time = candidate.Time
			} else if time.AsTime().After(candidates[i].Time.AsTime()) {
				candidate = candidates[i]
				time = candidate.Time
			}
		}
		finalCandidateId = int(candidate.Id)
		log.Printf("The candidate is peer %d", finalCandidateId)
		candidates = nil
		criticalUnlocked = false
	}
	savingCandidate.Unlock()
	return &pb.Reply{}, nil
}

func (p *peer) CriticalDone(ctx context.Context, peer *pb.Peer) (*pb.Reply, error) {
	Critical(peer)
	return &pb.Reply{}, nil
}

func Critical(peer *pb.Peer) {
	if finalCandidateId == int(peer.Id) {
		log.Printf("The critical section is available again")
		criticalUnlocked = true
	}
}
