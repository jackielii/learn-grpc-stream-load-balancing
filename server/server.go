package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"

	"learn-grpc-stream-load-balancing/pb"
	"learn-grpc-stream-load-balancing/queue"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type svc struct {
	id string // server id
	pb.UnimplementedMySvcServer

	queues sync.Map
}

func (s *svc) Stream(stream pb.MySvc_StreamServer) error {
	// log.Printf("client connected")
	ctx := stream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	// spew.Dump(md)

	clientId := md.Get("client_id")
	if len(clientId) == 0 {
		return fmt.Errorf("client_id not found")
	}
	log.Printf("server %s client %s connected", s.id, clientId[0])
	q, _ := s.queues.LoadOrStore(clientId[0], queue.New[*pb.HttpRequest, *pb.HttpResponse]())
	qu := q.(*queue.AsyncJobQueue[*pb.HttpRequest, *pb.HttpResponse])

	go qu.HandleSend(ctx, func(jr queue.JobReq[*pb.HttpRequest]) error {
		jr.Req.TraceId = jr.TraceId()
		return stream.Send(jr.Req)
	})

	go qu.HandleRecv(ctx, func() (queue.JobResp[*pb.HttpResponse], error) {
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
	clientId := in.GetClientId()
	q, ok := s.queues.Load(clientId)
	if !ok {
		return nil, fmt.Errorf("client %s not connected", clientId)
	}
	qu := q.(*queue.AsyncJobQueue[*pb.HttpRequest, *pb.HttpResponse])
	// log.Printf("server %s trigger %s", s.id, msg)
	resp, err := qu.Do(&pb.HttpRequest{N: msg})
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
		id: flag.Arg(0),
	})
	log.Printf("server %s listening at %v", flag.Arg(0), lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
