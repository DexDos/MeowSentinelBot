package service

import (
	"log"
	"strings"
	"time"
)

type Logger interface {
	panicHandle(error)
	tempErrorHandle(error, Severity)
	permErrorHandle(error, Severity)
	LogEvent(string, Severity)
}

type StdLogger struct {
	sb *strings.Builder
	t  *time.Time
	tz *time.Location
}

func NewStdLogger() *StdLogger {
	var sb strings.Builder
	t, err := time.Parse(time.UnixDate, "Wed Feb 25 11:06:39 PST 2015")
	if err != nil {
		panic(err)
	}

	tz, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		panic(err)
	}

	return &StdLogger{
		sb: &sb,
		t:  &t,
		tz: tz,
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

func (logger *StdLogger) panicHandle(err error) {
	logger.LogEvent(err.Error(), Error)
	panic(err)
}

func (logger *StdLogger) LogEvent(event string, t Severity) {
	log.Printf(
		"[%s] :: [%s]\n :: %s\n\n",
		t.String(),
		logger.t.In(logger.tz).Format(time.UnixDate),
		event,
	)
}

type Severity int

const (
	Fatal Severity = iota
	NonFatal
)

type Type int

func (t Severity) String() string {
	switch t {
	case Error:
		return "ERROR"
	case Warning:
		return "WARNING"
	case Info:
		return "INFO"
	case LifeCycle:
		return "LIFECYCLE"
	case Debug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

const (
	Error Severity = iota
	Warning
	Info
	LifeCycle
	Debug
)
