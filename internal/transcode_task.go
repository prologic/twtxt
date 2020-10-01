package internal

import (
	"errors"
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	ErrNoMediaFile = errors.New("error: no video or image file")
)

type TranscodeTask struct {
	*BaseTask

	conf *Config
	fn   string
}

func NewTranscodeTask(conf *Config, fn string) *TranscodeTask {
	return &TranscodeTask{
		BaseTask: &BaseTask{},

		conf: conf,
		fn:   fn,
	}
}

func (t *TranscodeTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	log.Infof("starting transcode task for %s", t.fn)

	opts := &VideoOptions{} // Resize: true, Size: MediaResolution}
	mediaURI, err := TranscodeVideo(t.conf, t.fn, mediaDir, "", opts)
	if err != nil {
		log.WithError(err).Errorf("error transcoding video %s", t.fn)
		return t.Fail(err)
	}
	log.Infof("transcode complete for %s with uri %s", t.fn, mediaURI)

	if err := os.Remove(t.fn); err != nil {
		log.WithError(err).Warn("error removing temporary video file")
	}

	t.SetData("mediaURI", mediaURI)

	return nil
}
