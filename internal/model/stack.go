package model

import "time"

type Stack struct {
	ID      int64
	OwnerId int64
	Title   string
	Created time.Time
	Updated time.Time
}
