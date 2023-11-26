package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"net"
	"sync"
	"time"

	gRPC "github.com/marcusaandahl/disys-hw5/proto"
)

var backupServer gRPC.AuctionHouseClient
var _ *grpc.ClientConn //the server connection
var isConnectedToBackup = false
var logMessagePrefix = "PRIMARY-SERVER"

type AuctionHouseServer struct {
	gRPC.UnimplementedAuctionHouseServer
	mutex       sync.Mutex // used to lock the server to avoid race conditions.
	serverState gRPC.ServerState
}

var port = 3000

func main() {
	launchServer()
}

func launchServer() {
	isBackup := false

	// Starts server on 3000 if primary - on 3001 as backup server if 3000 is occupied
	list, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", port))
	if err != nil {
		port = 3001
		list, err = net.Listen("tcp", fmt.Sprintf("localhost:%v", port))
		isBackup = true
		logMessagePrefix = "BACKUP-SERVER"
		check(err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	// Sets its default server state
	server := &AuctionHouseServer{
		serverState: gRPC.ServerState{
			EndAuctionTimestamp:   0,
			HighestBid:            0,
			HighestBidderId:       "",
			LastAuctionWinMessage: nil,
			IsBackupServer:        isBackup,
		},
	}

	gRPC.RegisterAuctionHouseServer(grpcServer, server) //Registers the server to the gRPC server.

	log.Println(fmt.Sprintf("(%v) Auction House started on port %v", logMessagePrefix, port))

	if err := grpcServer.Serve(list); err != nil {
		log.Fatalf("Failed to serve %v", err)
	}
}

func (s *AuctionHouseServer) Bid(_ context.Context, bidRequest *gRPC.BidRequest) (*gRPC.BidAck, error) {
	newMsg := ""
	if s.serverState.EndAuctionTimestamp > 0 && s.serverState.EndAuctionTimestamp <= time.Now().Unix() {
		// A bid has come in for an ended auction -> start new auction
		winnerMessage := setLastAuctionWinnerMessage(&s.serverState, s.serverState.HighestBid, s.serverState.HighestBidderId)
		s.serverState = gRPC.ServerState{
			EndAuctionTimestamp:   0,
			HighestBid:            0,
			HighestBidderId:       "",
			LastAuctionWinMessage: &winnerMessage,
			IsBackupServer:        s.serverState.GetIsBackupServer(),
		}

		newMsg = "The previous auction has ended, your bid request has unfortunately started a new auction...\n"
	}

	// Sets new end of auction timestamp
	if s.serverState.EndAuctionTimestamp == 0 {
		s.serverState.EndAuctionTimestamp = time.Now().Unix() + 100
	}

	if s.serverState.HighestBid < bidRequest.GetAmount() {
		// New winner bet
		s.serverState.HighestBid = bidRequest.GetAmount()
		s.serverState.HighestBidderId = bidRequest.GetUserId()
		go func() {
			// Non-blocking update backup server
			initiateServerUpdate(&s.serverState)
		}()

		return &gRPC.BidAck{
			State:   gRPC.ResponseState_SUCCESS,
			Message: fmt.Sprintf("%vYou are the current highest bidder with %v", newMsg, bidRequest.GetAmount()),
		}, nil
	} else {
		// Invalid bet (less than the highest bid)
		return &gRPC.BidAck{
			State:   gRPC.ResponseState_FAIL,
			Message: fmt.Sprintf("Higher bid exists (%v)", s.serverState.HighestBid),
		}, nil
	}
}

func (s *AuctionHouseServer) Result(_ context.Context, _ *emptypb.Empty) (*gRPC.ResultRes, error) {
	//Check if auction is over
	if s.serverState.GetEndAuctionTimestamp() > 0 && s.serverState.GetEndAuctionTimestamp() <= time.Now().Unix() {
		//The auction is over
		setLastAuctionWinnerMessage(&s.serverState, s.serverState.HighestBid, s.serverState.HighestBidderId)
		go func() {
			// Last auction winner message needs updating
			initiateServerUpdate(&s.serverState)
		}()
		return &gRPC.ResultRes{
			State:   gRPC.ResponseState_SUCCESS,
			Message: fmt.Sprintf("The auction is over!\n%v", s.serverState.GetLastAuctionWinMessage()),
		}, nil
	} else if s.serverState.GetEndAuctionTimestamp() > 0 && s.serverState.GetEndAuctionTimestamp() > time.Now().Unix() {
		// An auction is ongoing
		return &gRPC.ResultRes{
			State:   gRPC.ResponseState_SUCCESS,
			Message: fmt.Sprintf("The highest bid is %v by user %v\nTime remaining: %v seconds", s.serverState.GetHighestBid(), s.serverState.GetHighestBidderId(), s.serverState.GetEndAuctionTimestamp()-time.Now().Unix()),
		}, nil
	} else {
		// No auctions have started yet
		return &gRPC.ResultRes{
			State:   gRPC.ResponseState_SUCCESS,
			Message: "No auctions have yet to be run, submit a bid to start a new auction",
		}, nil
	}
}

func (s *AuctionHouseServer) UpdateServer(_ context.Context, serverState *gRPC.ServerState) (*gRPC.UpdateServerAck, error) {
	//Do not allow updates on the primary server
	if !s.serverState.GetIsBackupServer() {
		return &gRPC.UpdateServerAck{
			State: gRPC.ResponseState_FAIL,
		}, nil
	}

	// Updates the backup server's values
	s.serverState.HighestBid = serverState.GetHighestBid()
	s.serverState.HighestBidderId = serverState.GetHighestBidderId()
	s.serverState.EndAuctionTimestamp = serverState.GetEndAuctionTimestamp()
	s.serverState.LastAuctionWinMessage = serverState.LastAuctionWinMessage
	s.serverState.IsBackupServer = true

	return &gRPC.UpdateServerAck{
		State: gRPC.ResponseState_SUCCESS,
	}, nil
}

func initiateServerUpdate(serverState *gRPC.ServerState) {
	// If is backup server - do nothing
	if serverState.GetIsBackupServer() {
		return
	}

	// If server is not connected to backup - connect
	if !isConnectedToBackup {
		connectToBackup()
	}

	// Update backup server state
	res, err := backupServer.UpdateServer(context.Background(), serverState)

	if res.GetState() != gRPC.ResponseState_SUCCESS || err != nil {
		log.Println("Connection to backup is lost")
		isConnectedToBackup = false
	}
}

func setLastAuctionWinnerMessage(serverState *gRPC.ServerState, highestBid int32, winnerUser string) string {
	winnerMessage := fmt.Sprintf("Last auction was won with a bid of %v by user with ID %v", highestBid, winnerUser)
	serverState.LastAuctionWinMessage = &winnerMessage
	return winnerMessage
}

func connectToBackup() {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, _ := grpc.Dial(":3001", opts...)

	conBackup, cancelBackup := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelBackup()
	conn, _ = grpc.DialContext(conBackup, ":3001", opts...)

	backupServer = gRPC.NewAuctionHouseClient(conn)
	log.Printf("(%v) The connection is: %v\n", logMessagePrefix, conn.GetState().String())
	isConnectedToBackup = true
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
