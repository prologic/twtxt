package internal

type TaskState int

const (
	TaskStatePending TaskState = iota
	TaskStateRunning
	TaskStateComplete
	TaskStateFailed
)

func (t TaskState) String() string {
	switch t {
	case TaskStatePending:
		return "pending"
	case TaskStateRunning:
		return "running"
	case TaskStateComplete:
		return "complete"
	default:
		return "unknown"
	}
}

type TaskData map[string]string

type TaskResult struct {
	State TaskState `json:"state"`
	Error error     `json:"error"`
	Data  TaskData  `json:"data"`
}

// Task is an interface that represents a single task to be executed by a
// worker. Any object can implement a `Task` if it implements the interface.
type Task interface {
	State() TaskState
	Result() TaskResult
	Error() error
	Run() error
}
