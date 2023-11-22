package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"io"
	"log"
	"net"
	"sync"

	gRPC "github.com/marcusaandahl/disys-hw5/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

type SavedMessage struct {
	clientName string
	message    string
	join       bool
	leave      bool
	timestamp  int32
}

type Client struct {
	clientName string
}

type Server struct {
	gRPC.UnimplementedChatServer
	mutex    sync.Mutex // used to lock the server to avoid race conditions.
	messages []SavedMessage
	clients  []Client
}

var localTime int32 = 0

func main() {
	launchServer()
}

func launchServer() {
	log.Println("Server Chitty-Chat Attempts to create listener on port 5400")

	// Create listener tcp on given port or default port 5400
	list, err := net.Listen("tcp", "localhost:5400")
	if err != nil {
		log.Printf("Server Chitty-Chat Failed to listen on port 5400: %v", err) //If it fails to listen on the port, run launchServer method again with the next value/port in ports array
		return
	}

	// makes gRPC server using the options
	// you can add options here if you want or remove the options part entirely
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	// makes a new server instance using the name and port from the flags.
	server := &Server{}

	gRPC.RegisterChatServer(grpcServer, server) //Registers the server to the gRPC server.

	log.Printf("Server Chitty-Chat Listening at %v\n", list.Addr())

	if err := grpcServer.Serve(list); err != nil {
		log.Fatalf("failed to serve %v", err)
	}
}

func (s *Server) SendClientTransaction(_ context.Context, client *gRPC.ClientTransaction) (*emptypb.Empty, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	localTime = findMax(localTime, client.GetSenderTime()) + 1

	var action string

	if client.GetJoin() {
		action = "joined"
	} else {
		action = "left"
	}

	s.messages = append(s.messages, SavedMessage{
		clientName: client.GetClientName(),
		message:    fmt.Sprintf("Participant %v %v Chitty-Chat at Lamport time %v", client.GetClientName(), action, localTime),
		timestamp:  localTime,
	})

	return &emptypb.Empty{}, nil
}

func (s *Server) SendMessage(_ context.Context, msg *gRPC.Message) (*emptypb.Empty, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	localTime = findMax(localTime, msg.GetSenderTime()) + 1

	s.messages = append(s.messages, SavedMessage{
		clientName: msg.GetClientName(),
		message:    fmt.Sprintf("%v (Lamport %v): %v", msg.GetClientName(), localTime, msg.GetMessage()),
		timestamp:  localTime,
	})

	return &emptypb.Empty{}, nil
}

func (s *Server) GetBroadcast(_ *emptypb.Empty, msgStream gRPC.Chat_GetBroadcastServer) error {
	var messages = s.messages

	for _, message := range messages {
		localTime++
		err := msgStream.SendMsg(&gRPC.Message{
			ClientName: message.clientName,
			Message:    message.message,
			SenderTime: localTime,
		})
		// the stream is closed so we can exit the loop
		if err == io.EOF {
			break
		}
		// some other error
		if err != nil {
			return err
		}
	}

	for {
		if len(messages) == len(s.messages) {
			continue
		}

		for i := 0; i < (len(s.messages) - len(messages)); i++ {
			var messageToBroadcast = s.messages[len(messages)+i]

			messages = append(messages, messageToBroadcast)

			localTime++
			err := msgStream.SendMsg(&gRPC.Message{
				ClientName: messageToBroadcast.clientName,
				Message:    messageToBroadcast.message,
				SenderTime: localTime,
			})

			// the stream is closed so we can exit the loop
			if err == io.EOF {
				break
			}
			// some other error
			if err != nil {
				return err
			}
		}
	}
}

func findMax(a, b int32) int32 {
	if a > b {
		return a
	} else {
		return b
	}
}
