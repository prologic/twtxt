package internal

import (
	"fmt"
	"syscall"
)

type ErrCommandKilled struct {
	Err    error
	Signal syscall.Signal
}

func (e *ErrCommandKilled) Is(target error) bool {
	if _, ok := target.(*ErrCommandKilled); ok {
		return true
	}
	return false
}

func (e *ErrCommandKilled) Error() string {
	return fmt.Sprintf("error: command killed: %s", e.Err)
}

func (e *ErrCommandKilled) Unwrap() error {
	return e.Err
}

type ErrCommandFailed struct {
	Err    error
	Status int
}

func (e *ErrCommandFailed) Is(target error) bool {
	if _, ok := target.(*ErrCommandFailed); ok {
		return true
	}
	return false
}

func (e *ErrCommandFailed) Error() string {
	return fmt.Sprintf("error: command failed: %s", e.Err)
}

func (e *ErrCommandFailed) Unwrap() error {
	return e.Err
}

type ErrTranscodeTimeout struct {
	Err error
}

func (e *ErrTranscodeTimeout) Error() string {
	return fmt.Sprintf("error: transcode timed out: %s", e.Err)
}

func (e *ErrTranscodeTimeout) Unwrap() error {
	return e.Err
}

type ErrTranscodeFailed struct {
	Err error
}

func (e *ErrTranscodeFailed) Error() string {
	return fmt.Sprintf("error: transcode failed: %s", e.Err)
}

func (e *ErrTranscodeFailed) Unwrap() error {
	return e.Err
}

type ErrAudioUploadFailed struct {
	Err error
}

func (e *ErrAudioUploadFailed) Error() string {
	return fmt.Sprintf("error: audio upload failed: %s", e.Err)
}

func (e *ErrAudioUploadFailed) Unwrap() error {
	return e.Err
}

type ErrVideoUploadFailed struct {
	Err error
}

func (e *ErrVideoUploadFailed) Error() string {
	return fmt.Sprintf("error: video upload failed: %s", e.Err)
}

func (e *ErrVideoUploadFailed) Unwrap() error {
	return e.Err
}
