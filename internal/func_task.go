package internal

type FuncTask struct {
	*BaseTask

	f func() error
}

func NewFuncTask(f func() error) *FuncTask {
	return &FuncTask{
		BaseTask: &BaseTask{},

		f: f,
	}
}

func (t *FuncTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	return t.f()
}
