package service

import (
	"fmt"
	"log"
	"time"
)

type Logger interface {
	panicHandle(error)
	tempErrorHandle(t Severity, event LogErrorText, v ...any)
	permErrorHandle(t Severity, event LogErrorText, v ...any)
	LogEvent(t Severity, event LogErrorText, v ...any)
}

type StdLogger struct {
	tz *time.Location
}

func NewStdLogger() *StdLogger {
	tz, err := time.LoadLocation("Europe/Kyiv")
	if err != nil {
		tz = time.Local
	}

	return &StdLogger{
		tz: tz,
	}
}

func (logger *StdLogger) tempErrorHandle(t Severity, event LogErrorText, v ...any) {
	logger.LogEvent(t, event, v)
}

func (logger *StdLogger) permErrorHandle(t Severity, event LogErrorText, v ...any) {
	logger.LogEvent(t, event, v)
}

func (logger *StdLogger) panicHandle(err error) {
	logger.LogEvent(Fatal, PanicLog, err)
	panic(err)
}

func (logger *StdLogger) LogEvent(t Severity, event LogErrorText, v ...any) {
	currentTime := time.Now().In(logger.tz).Format(time.UnixDate)
	var errText string
	if len(v) == 0 {
		errText = string(event)
	} else {
		errText = fmt.Sprintf(
			string(event),
			v...,
		)
	}
	log.Printf(
		"\n]] :: [%s] :: [%s]\n :: %s\n\n",
		t.String(),
		currentTime,
		errText,
	)
}

type Severity int

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
	case Fatal:
		return "FATAL"
	case Attack:
		return "ATTACK"
	default:
		return "UNKNOWN"
	}
}

const (
	Fatal Severity = iota
	Error
	Warning
	Info
	LifeCycle
	Debug
	Attack
)

type LogErrorText string

const (
	PossibleAttack           LogErrorText = "Attack may have been detected. TID: [%d] | Suspicious payload: [%v]"
	BotStart                 LogErrorText = "Bot is starting"
	BotStop                  LogErrorText = "Bot is stopped"
	PanicLog                 LogErrorText = "error due to %v"
	InvalidNewStackNameLog   LogErrorText = "error while tried to create a stack %s for user id[%d]: %v"
	InvalidStackIDLog        LogErrorText = "error while trying to convert user input (%v) into integer. UserID[%d]: %v"
	MessageSendingFailureLog LogErrorText = "failed to send message to id[%v] %v due to %v"
	CreatingDBErrorLog       LogErrorText = "error creating db service: %v"
	DBErrorLog               LogErrorText = "error while db running: %v"
	UnknownError             LogErrorText = "unknown error: TID[%d] | Payload [%v] | Error [%v]"
)
