package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"learn-grpc-stream-load-balancing/pb"

	"google.golang.org/grpc"
)

type svc struct {
	id string // server id
	pb.UnimplementedMySvcServer

	req  chan string
	resp chan string
}

func (s *svc) Stream(stream pb.MySvc_StreamServer) error {
	log.Printf("client connected")
	go func() {
		for {
			log.Printf("server %s start receiving trigger", s.id)
			n := <-s.req
			log.Printf("server %s received trigger %s", s.id, n)
			log.Printf("server %s start sending request %s", s.id, n)
			stream.Send(&pb.HttpRequest{Id: s.id, N: n})
		}
	}()

	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}
		fmt.Printf("server %s received from client %s response %s\n", s.id, resp.Id, resp.N)
		s.resp <- resp.N
	}
}

func (s *svc) Trigger(ctx context.Context, in *pb.Trigger) (*pb.TriggerResponse, error) {
	log.Printf("server %s trigger %s", s.id, in.GetId())
	s.req <- in.GetId()
	resp := <-s.resp
	if resp != in.GetId() {
		log.Printf("server %s expected %s but got %s", s.id, in.GetId(), resp)
	}
	return &pb.TriggerResponse{
		Id: resp,
	}, nil
}

var port = flag.Int("port", 15432, "port")

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		return
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterMySvcServer(s, &svc{
		id:   flag.Arg(0),
		req:  make(chan string),
		resp: make(chan string),
	})
	log.Printf("server %s listening at %v", flag.Arg(0), lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
