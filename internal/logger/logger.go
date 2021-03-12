package logger

import (
	"fmt"

	"github.com/go-logr/logr"
)

type Logger struct {
	Log logr.Logger
}

func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args)
	l.Log.Info(msg)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args)
	l.Log.Error(nil, msg)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args)
	l.Log.V(10).Info(msg)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args)
	l.Log.V(5).Info(msg)
}
