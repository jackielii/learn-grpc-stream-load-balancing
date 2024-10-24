package main

import (
	"context"
	"flag"
	"log"

	"learn-grpc-stream-load-balancing/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = flag.String("addr", "localhost:15432", "the address to connect to")

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	// Set up a connection to the server.
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewMySvcClient(conn)

	ctx := context.Background()

	id := flag.Arg(0)

	stream, err := c.Stream(ctx)
	if err != nil {
		log.Fatalf("could not stream: %v", err)
	}
	for {
		log.Printf("client %s start receiving", id)
		req, err := stream.Recv()
		if err != nil {
			log.Fatalf("could not receive: %v", err)
		}

		log.Printf("client %s received %s", id, req.N)
		err = stream.Send(&pb.HttpResponse{Id: id, N: req.N})
		if err != nil {
			log.Fatalf("could not send: %v", err)
		}
	}
}
