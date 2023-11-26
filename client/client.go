package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/google/uuid"
	gRPC "github.com/marcusaandahl/disys-hw5/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var client gRPC.AuctionHouseClient
var clientConn *grpc.ClientConn //the server connection
var clientConnPort int32 = 3000

var ID = uuid.NewString()

func main() {
	//Connect to server
	connectToServer()

	// Closes safely if needed
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTSTP)
	go func() {
		<-signals
		Leave()
		os.Exit(0)
	}()

	log.Printf("(CLIENT-%v) Write 'bid' to bid and/or start an auction; or write 'result' to see the result of the latest auction.\n", ID)

	// Starts monitoring for input
	monitorInput()
}

func connectToServer() {
	// Attempts connection on port 3000 (primary)
	log.Printf("(CLIENT-%v) Starting a new connection\n", ID)
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conPrimary, cancelPrimary := context.WithTimeout(context.Background(), 3*time.Second)
	conn, err := grpc.DialContext(conPrimary, fmt.Sprintf(":%v", clientConnPort), opts...)

	if err != nil {
		// If fails on primary - try backup on port 3001
		cancelPrimary()

		clientConnPort = 3001

		conBackup, cancelBackup := context.WithTimeout(context.Background(), 3*time.Second)
		conn, err = grpc.DialContext(conBackup, fmt.Sprintf(":%v", clientConnPort), opts...)

		check(err)

		defer cancelBackup()
	}

	// Successful connection - generate client
	client = gRPC.NewAuctionHouseClient(conn)
	clientConn = conn
	log.Printf("(CLIENT-%v) The connection is: %v\n", ID, conn.GetState().String())

	cancelPrimary()
}

func Leave() {
	err := clientConn.Close()
	check(err)
}

func monitorInput() {
	var scanner = bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		userInput := scanner.Text()
		if strings.HasPrefix(userInput, "bid") {
			if len(userInput) > 5 {
				callBid(userInput[4:])
			} else {
				log.Printf("(CLIENT-%v) 'bid' command requires an amount\n", ID)
			}
		} else if userInput == "result" {
			callResult()
		} else {
			log.Printf("(CLIENT-%v) Invalid command - try 'result' or 'bid\n", ID)
		}
	}
}

func callBid(value string) {
	//Convert value to a integer
	amount, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("(CLIENT-%v) 'bid' command requires a valid number as amount\n", ID)
	}

	// Calls grpc method
	res, err := client.Bid(context.Background(), &gRPC.BidRequest{
		RequestId: uuid.NewString(),
		UserId:    ID,
		Amount:    int32(amount),
	})
	if err != nil || res.GetState() == gRPC.ResponseState_EXCEPTION {
		// Failed grpc method - falls back to backup server
		log.Printf("(CLIENT-%v) Server returned with exception - Attempting to switch to backup server for bid\n", ID)
		if clientConnPort == 3001 {
			log.Fatalf("(CLIENT-%v) Backup server failed\n", ID)
		}
		clientConnPort = 3001
		connectToServer()
		callBid(value)
	} else {
		log.Println(res.GetMessage())
	}
}

func callResult() {
	// Calls grpc method
	res, err := client.Result(context.Background(), &emptypb.Empty{})
	if err != nil || res.GetState() == gRPC.ResponseState_EXCEPTION {
		// Failed grpc method - falls back to backup server
		log.Printf("(CLIENT-%v) Server returned with exception - Attempting to switch server for result\n", ID)
		if clientConnPort == 3001 {
			log.Fatalf("(CLIENT-%v) Backup server failed\n", ID)
		}
		clientConnPort = 3001
		connectToServer()
		callResult()
	} else {
		log.Println(res.GetMessage())
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
