package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"learn-grpc-stream-load-balancing/pb"
	"learn-grpc-stream-load-balancing/queue"

	"google.golang.org/grpc"
)

type svc struct {
	id string // server id
	pb.UnimplementedMySvcServer

	queue *queue.AsyncJobQueue[*pb.HttpRequest, *pb.HttpResponse]
}

func (s *svc) Stream(stream pb.MySvc_StreamServer) error {
	// log.Printf("client connected")
	ctx := stream.Context()

	go s.queue.HandleSend(ctx, func(jr queue.JobReq[*pb.HttpRequest]) error {
		jr.Req.TraceId = jr.TraceId()
		return stream.Send(jr.Req)
	})

	go s.queue.HandleRecv(ctx, func() (queue.JobResp[*pb.HttpResponse], error) {
		resp, err := stream.Recv()
		if err != nil {
			return queue.JobResp[*pb.HttpResponse]{}, err
		}
		jr := queue.JobResp[*pb.HttpResponse]{Resp: resp}
		jr.SetTraceId(resp.TraceId)
		return jr, nil
	})

	<-ctx.Done()

	return nil
}

func (s *svc) Trigger(ctx context.Context, in *pb.Trigger) (*pb.TriggerResponse, error) {
	msg := in.GetMsg()
	// log.Printf("server %s trigger %s", s.id, msg)
	resp, err := s.queue.Do(&pb.HttpRequest{N: msg})
	return &pb.TriggerResponse{Msg: resp.N}, err
}

var port = flag.Int("port", 54321, "port")

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
		id:    flag.Arg(0),
		queue: queue.New[*pb.HttpRequest, *pb.HttpResponse](),
	})
	log.Printf("server %s listening at %v", flag.Arg(0), lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
