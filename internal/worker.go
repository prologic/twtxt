package internal

import (
	log "github.com/sirupsen/logrus"
)

// Worker attaches to a provided worker pool, and
// looks for tasks on its task channel
type Worker struct {
	workerPool  chan chan Task
	taskChannel chan Task
	quit        chan bool
}

// NewWorker creates a new worker using the given id and
// attaches to the provided worker pool. It also initializes
// the task/quit channels
func NewWorker(workerPool chan chan Task) *Worker {
	return &Worker{
		workerPool:  workerPool,
		taskChannel: make(chan Task),
		quit:        make(chan bool),
	}
}

// Start initializes a select loop to listen for tasks to execute
func (w *Worker) Start() {
	go func() {
		for {
			w.workerPool <- w.taskChannel

			select {
			case task := <-w.taskChannel:
				if err := task.Run(); err != nil {
					log.WithError(err).Errorf("error running task %q", task)
				}
			case <-w.quit:
				return
			}
		}
	}()
}

// Stop will end the task select loop for the worker
func (w *Worker) Stop() {
	go func() {
		w.quit <- true
	}()
}
