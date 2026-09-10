package model

import "time"

type Entry struct {
	Id      int64
	StackId int64
	Title   string
	Type    string
	Status  Status
	Created time.Time
	Updated time.Time
}
