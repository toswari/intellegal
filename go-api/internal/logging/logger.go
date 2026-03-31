package logging

import logger "github.com/Gratheon/log-lib-go"

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Adapter struct {
	discard bool
}

func New(_ any) *Adapter {
	return &Adapter{}
}

func NewDiscard(_ any) *Adapter {
	return &Adapter{discard: true}
}

func (a *Adapter) Info(msg string, args ...any) {
	if a == nil || a.discard {
		return
	}
	logger.Info(msg, args...)
}

func (a *Adapter) Warn(msg string, args ...any) {
	if a == nil || a.discard {
		return
	}
	logger.Warn(msg, args...)
}

func (a *Adapter) Error(msg string, args ...any) {
	if a == nil || a.discard {
		return
	}
	logger.Error(msg, args...)
}
