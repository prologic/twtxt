package internal

type BaseTask struct {
	state TaskState
	data  TaskData
	err   error
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
	return TaskResult{
		State: t.state,
		Error: t.err,
		Data:  t.data,
	}
}

func (t *BaseTask) State() TaskState { return t.state }
func (t *BaseTask) Error() error     { return t.err }
