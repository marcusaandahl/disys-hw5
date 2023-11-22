package main

import (
	"bufio"
	"context"
	"github.com/inancgumus/screen"
	gRPC "github.com/marcusaandahl/disys-hw5/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
	"os"
	"os/signal"
	"os/user"
	"syscall"
)

var server gRPC.ChatClient
var ServerConn *grpc.ClientConn //the server connection

var messages []*gRPC.Message

var localTime int32 = 0

func main() {
	ConnectToServer()

	sysUser, err := user.Current()

	stream, err := server.GetBroadcast(context.Background(), &emptypb.Empty{})

	if err != nil {
		log.Fatalln("Unable to start up")
	}

	signals := make(chan os.Signal, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTSTP)

	go func() {
		<-signals
		Leave(sysUser.Name, stream)
		os.Exit(0)
	}()

	FollowBroadcast(sysUser.Name, stream)
}

func ConnectToServer() {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.Dial(":5400", opts...)
	if err != nil {
		log.Fatalf("Fail to Dial : %v", err)
		return
	}

	server = gRPC.NewChatClient(conn)
	ServerConn = conn
	log.Println("The connection is: ", conn.GetState().String())
}

func Leave(userName string, stream gRPC.Chat_GetBroadcastClient) {
	SendLeave(userName)

	err := stream.CloseSend()
	err = ServerConn.Close()

	if err != nil {
		log.Fatalln("Unable to close stream")
	}
}

func SendJoin(userName string) {
	localTime++
	client := &gRPC.ClientTransaction{
		ClientName: userName,
		Join:       true,
		SenderTime: localTime,
	}

	_, err := server.SendClientTransaction(context.Background(), client)
	if err != nil {
		log.Print("Client: no response from the server, attempting to reconnect: ")
		log.Println(err)
	}
}

func SendLeave(userName string) {
	localTime++
	client := &gRPC.ClientTransaction{
		ClientName: userName,
		Join:       false,
		SenderTime: localTime,
	}

	_, err := server.SendClientTransaction(context.Background(), client)
	if err != nil {
		log.Print("Client: no response from the server, attempting to reconnect: ")
		log.Println(err)
	}
}

func SendMessage(userName string, messageInput string) {

	if len(messageInput) > 128 {
		log.Println("Message can max be 128 characters long! Message was not sent")
		log.Print(userName + "> ")
		return
	}

	localTime++
	message := &gRPC.Message{
		ClientName: userName,
		Message:    messageInput,
		Timestamp:  localTime,
		SenderTime: localTime,
	}

	_, err := server.SendMessage(context.Background(), message)
	if err != nil {
		log.Print("Client: no response from the server, attempting to reconnect: ")
		log.Println(err)
	}
}

func FollowBroadcast(userName string, stream gRPC.Chat_GetBroadcastClient) {
	screen.Clear()

	SendJoin(userName)

	go func() {
		for {
			msg, err := stream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				log.Fatalf("Failed to receive a note : %v", err)
			}

			localTime = findMax(localTime, msg.GetSenderTime()) + 1

			messages = append(messages, msg)

			screen.Clear()
			screen.MoveTopLeft()
			for _, message := range messages {
				log.Println(message.GetMessage())
			}

			log.Printf("%v> ", userName)
		}
	}()

	var scanner = bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		SendMessage(userName, scanner.Text())
	}
}

func findMax(a, b int32) int32 {
	if a > b {
		return a
	} else {
		return b
	}
}
