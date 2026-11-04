// Package notify runs the background worker: every tick it runs each registered job
// (expire overdue requests, send reminders, expire subscriptions, ...).
package notify

import (
	"context"
	"log/slog"
	"time"
)

// Job is one step of a worker run. It returns how many items it handled.
type Job interface {
	Name() string
	Run(ctx context.Context) (int, error)
}

// JobFunc adapts a function to Job.
type JobFunc struct {
	JobName string
	Fn      func(ctx context.Context) (int, error)
}

func (j JobFunc) Name() string                         { return j.JobName }
func (j JobFunc) Run(ctx context.Context) (int, error) { return j.Fn(ctx) }

type Worker struct {
	jobs     []Job
	interval time.Duration
	log      *slog.Logger
}

func NewWorker(interval time.Duration, log *slog.Logger, jobs ...Job) *Worker {
	return &Worker{jobs: jobs, interval: interval, log: log}
}

// Add registers more jobs; call before Run.
func (w *Worker) Add(jobs ...Job) { w.jobs = append(w.jobs, jobs...) }

// Run ticks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("worker started", slog.Duration("interval", w.interval), slog.Int("jobs", len(w.jobs)))
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			w.log.Info("worker stopped")
			return nil
		case <-t.C:
			w.RunOnce(ctx)
		}
	}
}

// RunOnce runs every job once, in order. A failing job is logged and does not stop the others.
func (w *Worker) RunOnce(ctx context.Context) map[string]int {
	res := make(map[string]int, len(w.jobs))
	for _, j := range w.jobs {
		n, err := j.Run(ctx)
		res[j.Name()] = n
		switch {
		case err != nil:
			w.log.ErrorContext(ctx, "worker job failed", slog.String("job", j.Name()), slog.Int("handled", n), slog.Any("err", err))
		case n > 0:
			w.log.InfoContext(ctx, "worker job", slog.String("job", j.Name()), slog.Int("handled", n))
		}
	}
	return res
}
