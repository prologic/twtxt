package internal

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

type AudioTask struct {
	*BaseTask

	conf *Config
	fn   string
}

func NewAudioTask(conf *Config, fn string) *AudioTask {
	return &AudioTask{
		BaseTask: NewBaseTask(),

		conf: conf,
		fn:   fn,
	}
}

func (t *AudioTask) String() string { return fmt.Sprintf("%T: %s", t, t.ID()) }
func (t *AudioTask) Run() error {
	defer t.Done()
	t.SetState(TaskStateRunning)

	log.Infof("starting audio transcode task for %s", t.fn)

	opts := &AudioOptions{
		Resample:   true,
		Channels:   1,
		Samplerate: 16000,
		Bitrate:    96,
	}
	mediaURI, err := TranscodeAudio(t.conf, t.fn, mediaDir, "", opts)
	if err != nil {
		log.WithError(err).Errorf("error transcoding audio %s", t.fn)
		return t.Fail(err)
	}
	log.Infof("audio transcode complete for %s with uri %s", t.fn, mediaURI)

	if err := os.Remove(t.fn); err != nil {
		log.WithError(err).Warn("error removing temporary audio file")
	}

	t.SetData("mediaURI", mediaURI)

	return nil
}
