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

	// Set up a connection to the server.
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewMySvcClient(conn)

	ctx := context.Background()

	if flag.NArg() == 0 {
		flag.Usage()
		return
	}

	resp, err := c.Trigger(ctx, &pb.Trigger{Id: flag.Arg(0)})
	if err != nil {
		log.Fatalf("could not trigger: %v", err)
	}
	if resp.GetId() != flag.Arg(0) {
		log.Fatalf("expected %s but got %s", flag.Arg(0), resp.GetId())
	}
}
