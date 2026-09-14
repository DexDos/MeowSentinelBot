package model

import "time"

type StackInfo struct {
	ID      int64
	OwnerId int64
	Title   string
	Created time.Time
	Updated time.Time
}

type Stack struct {
	Info    *StackInfo
	Entries []Entry
}

func NewStack(info *StackInfo, entries []Entry) Stack {
	return Stack{
		Info:    info,
		Entries: entries,
	}
}
