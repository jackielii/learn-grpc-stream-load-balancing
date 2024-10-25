// Description: This package provides a simple async job queue that can be used in a gRPC stream server to control job requests and responses.
//
// Example:
//
//	func (s *svc) Stream(stream pb.MySvc_StreamServer) error {
//	  ctx := stream.Context()
//
//	  go s.queue.HandleSend(ctx, func(jr queue.JobReq[*pb.HttpRequest]) error {
//	    jr.Req.TraceId = jr.TraceId()
//	    return stream.Send(jr.Req)
//	  })
//
//	  go s.queue.HandleRecv(ctx, func() (queue.JobResp[*pb.HttpResponse], error) {
//	    resp, err := stream.Recv()
//	    if err != nil {
//	      return queue.JobResp[*pb.HttpResponse]{}, err
//	    }
//	    jr := queue.JobResp[*pb.HttpResponse]{Resp: resp}
//	    jr.SetTraceId(resp.TraceId)
//	    return jr, nil
//	  })
//
//	  <-ctx.Done()
//
//	  return nil
//	}
//
//	func (s *svc) Trigger(ctx context.Context, in *pb.Trigger) (*pb.TriggerResponse, error) {
//	  msg := in.GetMsg()
//	  resp, err := s.queue.Do(&pb.HttpRequest{N: msg})
//	  return &pb.TriggerResponse{Msg: resp.N}, err
//	}
package queue

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
)

type JobReq[T any] struct {
	traceId int64
	Req     T
}

func (jr JobReq[T]) TraceId() int64 {
	return jr.traceId
}

type JobResp[R any] struct {
	traceId int64
	Resp    R
}

func (jr *JobResp[R]) SetTraceId(traceId int64) {
	jr.traceId = traceId
}

type resultChan[R any] struct {
	traceId int64
	respCh  chan R
}

type AsyncJobQueue[T any, R any] struct {
	req         chan JobReq[T]
	respChanMap map[int64]resultChan[R]
	mu          sync.Mutex
}

// New creates a new AsyncJobQueue
func New[T, R any]() *AsyncJobQueue[T, R] {
	queue := &AsyncJobQueue[T, R]{
		req:         make(chan JobReq[T]),
		respChanMap: make(map[int64]resultChan[R]),
	}
	return queue
}

// HandleSend gets the request from the queue and calls send function
func (q *AsyncJobQueue[T, R]) HandleSend(ctx context.Context, send func(JobReq[T]) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case req := <-q.req:
			if err := send(req); err != nil {
				// log.Printf("send error: %v", err)
				return err
			}
		}
	}
}

// HandleRecv gets a message from the stream and sends it to the queue for the result to be picked up by the Do
// recv should return io.EOF when the client disconnects
func (q *AsyncJobQueue[T, R]) HandleRecv(ctx context.Context, recv func() (JobResp[R], error)) error {
	for {
		// recv should return io.EOF when the client disconnects so we don't need to check ctx.Err()
		// if err := ctx.Err(); err != nil {
		// 	log.Printf("context error: %v", err)
		// 	return err
		// }
		resp, err := recv() // recv should return io.EOF when the client disconnects
		if err != nil {
			log.Printf("recv error: %v", err)
			return err
		}
		if resp.traceId == 0 {
			panic("you must set trace id in recv func, make sure trace id is passed through req and resp")
		}
		// log.Printf("response trace id: %d, msg: %v", resp.traceId, resp)
		q.mu.Lock()
		rc, ok := q.respChanMap[resp.traceId]
		if !ok {
			// trace id channel has to be setup before sending the request
			panic("unexpected trace id, channel is not setup, fix code")
		}
		q.mu.Unlock()
		rc.respCh <- resp.Resp
	}
}

// Do sends the request to the queue and waits for the response, then returns the response
func (q *AsyncJobQueue[T, R]) Do(r T) (R, error) {
	traceId := rand.Int64()
	respCh := make(chan R)
	q.mu.Lock()
	q.respChanMap[traceId] = resultChan[R]{
		traceId: traceId,
		respCh:  respCh,
	}
	q.mu.Unlock()
	q.req <- JobReq[T]{
		traceId: traceId,
		Req:     r,
	}

	resp := <-respCh
	q.mu.Lock()
	delete(q.respChanMap, traceId)
	q.mu.Unlock()
	return resp, nil
}
