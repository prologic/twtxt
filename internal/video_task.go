package internal

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

type VideoTask struct {
	*BaseTask

	conf *Config
	fn   string
}

func NewVideoTask(conf *Config, fn string) *VideoTask {
	return &VideoTask{
		BaseTask: NewBaseTask(),

		conf: conf,
		fn:   fn,
	}
}

func (t *VideoTask) String() string { return fmt.Sprintf("%T: %s", t, t.ID()) }
func (t *VideoTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	log.Infof("starting video transcode task for %s", t.fn)

	opts := &VideoOptions{} // Resize: true, Size: MediaResolution}
	mediaURI, err := TranscodeVideo(t.conf, t.fn, mediaDir, "", opts)
	if err != nil {
		log.WithError(err).Errorf("error transcoding video %s", t.fn)
		return t.Fail(err)
	}
	log.Infof("video transcode complete for %s with uri %s", t.fn, mediaURI)

	if err := os.Remove(t.fn); err != nil {
		log.WithError(err).Warn("error removing temporary video file")
	}

	t.SetData("mediaURI", mediaURI)

	return nil
}
