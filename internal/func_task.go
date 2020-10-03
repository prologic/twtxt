package internal

import "fmt"

type FuncTask struct {
	*BaseTask

	f func() error
}

func NewFuncTask(f func() error) *FuncTask {
	return &FuncTask{
		BaseTask: NewBaseTask(),

		f: f,
	}
}

func (t *FuncTask) String() string { return fmt.Sprintf("%T: %s", t, t.ID()) }
func (t *FuncTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	return t.f()
}
