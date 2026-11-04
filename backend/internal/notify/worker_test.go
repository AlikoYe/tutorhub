package notify

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestRunOnceRunsEveryJobEvenAfterAFailure(t *testing.T) {
	var order []string
	job := func(name string, n int, err error) Job {
		return JobFunc{JobName: name, Fn: func(context.Context) (int, error) {
			order = append(order, name)
			return n, err
		}}
	}
	w := NewWorker(time.Minute, slog.New(slog.DiscardHandler), job("a", 1, errors.New("boom")))
	w.Add(job("b", 2, nil))
	res := w.RunOnce(context.Background())
	if len(order) != 2 || order[0] != "a" || order[1] != "b" || res["a"] != 1 || res["b"] != 2 {
		t.Fatalf("order %v, result %v", order, res)
	}
}

func TestRunStopsOnCancel(t *testing.T) {
	ran := make(chan struct{}, 10)
	w := NewWorker(5*time.Millisecond, slog.New(slog.DiscardHandler), JobFunc{JobName: "tick", Fn: func(context.Context) (int, error) {
		ran <- struct{}{}
		return 0, nil
	}})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() { done <- w.Run(ctx) }()
	<-ran
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
