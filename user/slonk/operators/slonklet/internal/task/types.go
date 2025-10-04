package task

import (
	"context"
	"os"
)

type Entry interface {
	Start(ctx context.Context, sigChan chan os.Signal, doneChan chan bool) error
	Kill() error
}

type Queue interface {
	Enqueue(entry Entry) (string, error)
	List(entry Entry) ([]Entry, error)
	Get(id string) (Entry, error)
}
