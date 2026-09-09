package service

import "strings"

type Logger interface {
	panicHandle(error)
	tempErrorHandle(error, Severity)
	permErrorHandle(error, Severity)
}

type StdLogger struct {
	sb *strings.Builder
}

func NewStdLogger() *StdLogger {
	var sb strings.Builder
	return &StdLogger{
		sb: &sb,
	}
}

func (logger *StdLogger) tempErrorHandle(err error, severity Severity) {
	//TODO implement me
	panic("implement me")
}

func (logger *StdLogger) permErrorHandle(err error, severity Severity) {
	//TODO implement me
	panic("implement me")
}

func (logger *StdLogger) panicHandle(err error) {}

type Severity int

const (
	Fatal Severity = iota
	NonFatal
)
