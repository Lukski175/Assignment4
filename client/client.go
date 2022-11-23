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
var waitingForAccess bool = false
var proposing bool = false

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

	// For how many clients we expect, dial each peer
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

		log.Printf("Clients connected: %d", len(p.clients))
	}
	log.Printf("Connection established")
	log.Printf("")

	// Client loop
	for {
		// If client isnt waiting for its access to critical
		if !waitingForAccess {
			// Request access
			pear := &pb.Peer{Id: p.id, Time: timestamppb.Now()}
			waitingForAccess = true
			for _, cl := range p.clients {
				_, err := cl.RequestAccess(ctx, pear)
				if err != nil {
					log.Printf("%s", err)
				}
			}
			p.Access(pear)
		}

		if criticalUnlocked {
			// If critical section is unlocked, ill propose my candidate to enter
			if len(p.criticalQueue) > 0 {
				proposing = true
				criticalUnlocked = false
				p.DoPropose(ctx)
			}
		}
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
	p.Access(peer)

	return &pb.Reply{}, nil
}

func (p *peer) Access(peer *pb.Peer) {
	// If the client wants to access critical, append to the queue
	p.criticalQueue = append(p.criticalQueue, peer)
}

var proposedAmount int
var savingCandidate sync.Mutex
var candidates = make([]*pb.Peer, 0)
var finalCandidateId int

func (p *peer) DoPropose(ctx context.Context) {

	// Propose candidate
	var lowestTime *timestamppb.Timestamp
	var candidate *pb.Peer
	var criticalQueuePrint string
	criticalQueuePrint = "Critical Queue: "
	for i := 0; i < len(p.criticalQueue); i++ {
		criticalQueuePrint += strconv.Itoa(int(p.criticalQueue[i].Id)) + " "

		// Loop through each client to determine the client with the lowest time, which has priority
		if i == 0 || lowestTime.AsTime().After(p.criticalQueue[i].Time.AsTime()) {
			candidate = p.criticalQueue[i]
			lowestTime = candidate.Time
		}
	}
	log.Printf(criticalQueuePrint)

	log.Printf("I propose: %d", candidate.Id)
	pear := &pb.Peer{Id: candidate.Id, Time: lowestTime}
	for _, cl := range p.clients {
		//Proposes candidate to other clients
		_, err := cl.ProposeCandidate(ctx, pear)
		if err != nil {
			log.Printf("%s", err)
		}
	}
	//Proposes candidate to self
	p.ProposeCandidate(ctx, pear)
}

func (p *peer) ProposeCandidate(ctx context.Context, peer *pb.Peer) (*pb.Reply, error) {
	// This is needed, unsure why
	time.Sleep(2 * time.Second)

	savingCandidate.Lock()
	proposedAmount++
	candidates = append(candidates, peer)
	if proposedAmount >= numberOfClients {
		// Calculates final candidate
		var time *timestamppb.Timestamp
		var candidate *pb.Peer
		candidatePrint := "Proposed candidates: "
		for i := 0; i < len(candidates); i++ {
			candidatePrint += strconv.Itoa(int(candidates[i].Id)) + " "
			if i == 0 || time.AsTime().After(candidates[i].Time.AsTime()) {
				candidate = candidates[i]
				time = candidate.Time
			}
		}
		log.Printf(candidatePrint)
		finalCandidateId = int(candidate.Id)
		log.Printf("Final candidate: %d", finalCandidateId)

		// Resets values to make sure it runs next time
		candidates = make([]*pb.Peer, 0)
		proposing = false
		proposedAmount = 0
		// Removes final candidate from critical queue
		p.criticalQueue = Remove(p.criticalQueue, candidate)

		// If i am the client to enter critical, i enter critical
		// Every client has same code, so should be same for all
		if finalCandidateId == int(p.id) {
			p.DoCritical(ctx)
		} else {
			log.Printf("Client %d is entering the critical section", finalCandidateId)
		}
	}
	savingCandidate.Unlock()
	return &pb.Reply{}, nil
}

// Code where client in critical "executes" critical section (aka waits 5 seconds)
func (p *peer) DoCritical(ctx context.Context) {
	log.Print("Entering critical...")

	time.Sleep(5 * time.Second) // Critical section :D

	log.Print("Done with critical")

	// Tell every client i am done with critical
	pear := &pb.Peer{Id: p.id}
	for _, cl := range p.clients {
		_, err := cl.CriticalDone(ctx, pear)
		if err != nil {
			log.Printf("%s", err)
		}
	}
	Critical(pear)
	waitingForAccess = false
}

// Called on every client after critical is done
// Just calls critical on self
func (p *peer) CriticalDone(ctx context.Context, peer *pb.Peer) (*pb.Reply, error) {
	Critical(peer)
	return &pb.Reply{}, nil
}

func Critical(peer *pb.Peer) {
	log.Printf("Client %d is trying to open critical.", peer.Id)
	// Validates if correct peer is trying to open critical
	if finalCandidateId == int(peer.Id) {
		log.Printf("Access granted")
		// Opens critical
		criticalUnlocked = true
		log.Printf("The critical section is available again")
		log.Printf("")
	}
}

// Function to remove a peer from critical queue
func Remove(list []*pb.Peer, peer *pb.Peer) []*pb.Peer {
	returnList := make([]*pb.Peer, 0)

	for index, _ := range list {
		p := list[index]
		if p.Id != peer.Id {
			returnList = append(returnList, list[index])
		}
	}
	return returnList
}
