package recorders

import "errors"

var (
	ErrRecorderExist          = errors.New("SimpleRecorder is exist")
	ErrRecorderNotExist       = errors.New("SimpleRecorder is not exist")
	ErrParserNotSupportStatus = errors.New("parser not support get status")
)
