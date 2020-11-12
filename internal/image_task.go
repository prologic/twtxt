package internal

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

type ImageTask struct {
	*BaseTask

	conf *Config
	fn   string
}

func NewImageTask(conf *Config, fn string) *ImageTask {
	return &ImageTask{
		BaseTask: NewBaseTask(),

		conf: conf,
		fn:   fn,
	}
}

func (t *ImageTask) String() string { return fmt.Sprintf("%T: %s", t, t.ID()) }
func (t *ImageTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	log.Infof("starting image processing task for %s", t.fn)

	opts := &ImageOptions{Resize: true, Width: MediaResolution, Height: 0}
	mediaURI, err := ProcessImage(t.conf, t.fn, mediaDir, "", opts)
	if err != nil {
		log.WithError(err).Errorf("error processing image %s", t.fn)
		return t.Fail(err)
	}
	log.Infof("image processing complete for %s with uri %s", t.fn, mediaURI)

	if err := os.Remove(t.fn); err != nil {
		log.WithError(err).Warn("error removing temporary image file")
	}

	t.SetData("mediaURI", mediaURI)

	return nil
}
