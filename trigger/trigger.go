package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"sync"

	"learn-grpc-stream-load-balancing/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var addr = flag.String("addr", "localhost:54321", "the address to connect to")

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

	if flag.NArg() < 2 {
		flag.Usage()
		return
	}

	clientId := flag.Arg(0)
	triggerMsg := flag.Arg(1)

	n := 1
	if flag.NArg() > 2 {
		n, err = strconv.Atoi(flag.Arg(2))
		if err != nil {
			log.Fatal(err)
		}
	}

	wg := sync.WaitGroup{}
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			msg := fmt.Sprintf("%s: %d", triggerMsg, i)
			req := &pb.Trigger{
				ClientId: clientId,
				Msg:      msg,
			}
			resp, err := c.Trigger(ctx, req)
			if err != nil {
				log.Printf("could not trigger: %v", err)
			} else if resp.GetMsg() != msg {
				log.Printf("expected %s but got %s", msg, resp.GetMsg())
			}
		}()
	}
	wg.Wait()
}
