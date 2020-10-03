package internal

import (
	"fmt"

	"github.com/renstrom/shortuuid"
)

type BaseTask struct {
	state TaskState
	data  TaskData
	err   error
	id    string
}

func NewBaseTask() *BaseTask {
	return &BaseTask{
		data: make(TaskData),
		id:   shortuuid.New(),
	}
}

func (t *BaseTask) SetState(state TaskState) {
	t.state = state
}

func (t *BaseTask) SetData(key, val string) {
	if t.data == nil {
		t.data = make(TaskData)
	}
	t.data[key] = val
}

func (t *BaseTask) Done() {
	if t.err != nil {
		t.state = TaskStateFailed
	} else {
		t.state = TaskStateComplete
	}
}

func (t *BaseTask) Fail(err error) error {
	t.err = err
	return err
}

func (t *BaseTask) Result() TaskResult {
	stateStr := t.state.String()
	errStr := ""
	if t.err != nil {
		errStr = t.err.Error()
	}

	return TaskResult{
		State: stateStr,
		Error: errStr,
		Data:  t.data,
	}
}

func (t *BaseTask) String() string   { return fmt.Sprintf("%T: %s", t, t.ID()) }
func (t *BaseTask) ID() string       { return t.id }
func (t *BaseTask) State() TaskState { return t.state }
func (t *BaseTask) Error() error     { return t.err }
