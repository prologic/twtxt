package internal

import (
	"errors"
)

// Dispatcher maintains a pool for available workers
// and a task queue that workers will process
type Dispatcher struct {
	maxWorkers int
	maxQueue   int
	workers    []*Worker
	workerPool chan chan Task
	taskQueue  chan Task
	taskMap    map[string]Task
	quit       chan bool
	active     bool
}

// NewDispatcher creates a new dispatcher with the given
// number of workers and buffers the task queue based on maxQueue.
// It also initializes the channels for the worker pool and task queue
func NewDispatcher(maxWorkers int, maxQueue int) *Dispatcher {
	return &Dispatcher{
		maxWorkers: maxWorkers,
		maxQueue:   maxQueue,
	}
}

// Start creates and starts workers, adding them to the worker pool.
// Then, it starts a select loop to wait for tasks to be dispatched
// to available workers
func (d *Dispatcher) Start() {
	d.workers = []*Worker{}
	d.workerPool = make(chan chan Task, d.maxWorkers)
	d.taskQueue = make(chan Task, d.maxQueue)
	d.taskMap = make(map[string]Task)
	d.quit = make(chan bool)

	for i := 0; i < d.maxWorkers; i++ {
		worker := NewWorker(d.workerPool)
		worker.Start()
		d.workers = append(d.workers, worker)
	}

	d.active = true

	go func() {
		for {
			select {
			case task := <-d.taskQueue:
				go func(task Task) {
					taskChannel := <-d.workerPool
					taskChannel <- task
				}(task)
			case <-d.quit:
				return
			}
		}
	}()
}

// Stop ends execution for all workers and closes all channels, then removes
// all workers
func (d *Dispatcher) Stop() {
	if !d.active {
		return
	}

	d.active = false

	for i := range d.workers {
		d.workers[i].Stop()
	}

	d.workers = []*Worker{}
	d.quit <- true
}

// Lookup returns the matching `Task` given its id
func (d *Dispatcher) Lookup(id string) (Task, bool) {
	task, ok := d.taskMap[id]
	return task, ok
}

// Dispatch pushes the given task into the task queue.
// The first available worker will perform the task
func (d *Dispatcher) Dispatch(task Task) (string, error) {
	if !d.active {
		return "", errors.New("dispatcher is not active")
	}

	d.taskQueue <- task
	d.taskMap[task.ID()] = task
	return task.ID(), nil
}

// DispatchFunc pushes the given func into the task queue by first wrapping
// it with a `TaskFunc` task.
func (d *Dispatcher) DispatchFunc(f func() error) (string, error) {
	return d.Dispatch(NewFuncTask(f))
}
