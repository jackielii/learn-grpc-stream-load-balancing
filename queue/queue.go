package queue

import (
	"context"
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

func New[T, R any]() *AsyncJobQueue[T, R] {
	queue := &AsyncJobQueue[T, R]{
		req:         make(chan JobReq[T]),
		respChanMap: make(map[int64]resultChan[R]),
	}
	return queue
}

func (q *AsyncJobQueue[T, R]) HandleSend(ctx context.Context, send func(JobReq[T]) error) error {
	for req := range q.req {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := send(req); err != nil {
			return err
		}
	}
	return nil
}

func (q *AsyncJobQueue[T, R]) HandleRecv(ctx context.Context, recv func() (JobResp[R], error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		resp, err := recv()
		if err != nil {
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
