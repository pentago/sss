package worker

import (
	"context"
	"testing"
	"time"

	"sss/internal/model"
)

func TestShutdownUnblocksPendingPush(t *testing.T) {
	pool := NewPool(context.Background(), nil, 0)

	pushed := make(chan struct{})
	go func() {
		defer close(pushed)
		pool.Push(model.JobList{{FeedID: 1}})
	}()
	time.Sleep(10 * time.Millisecond) // let Push block on the unbuffered queue

	pool.Shutdown()
	select {
	case <-pushed:
	case <-time.After(5 * time.Second):
		t.Fatal("Push remained blocked after Shutdown")
	}
	pool.Shutdown() // idempotent
}
