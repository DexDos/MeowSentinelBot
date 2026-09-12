package service

import "strings"

type Responder interface {
	RespondRandomly() string
}

type StdResponder struct {
	sb *strings.Builder
}

func NewStdResponder() *StdResponder {
	return &StdResponder{sb: new(strings.Builder)}
}

func (r *StdResponder) RespondRandomly() string {
	return ""
}
