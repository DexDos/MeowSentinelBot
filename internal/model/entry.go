package model

import (
	"fmt"
	"time"
)

type Entry struct {
	Id      int64
	StackId int64
	Title   string
	Type    string
	Status  Status
	Created time.Time
	Updated time.Time
}

func (e Entry) String() string {
	return fmt.Sprintf(
		"[%d]**%s**: %v\n",
		e.Id,
		e.Title,
		e.Status,
	)
}

type Status struct {
	Season          int64
	LastEpisodeSeen int64
	Rating          Rating
}

func (s Status) String() string {
	return fmt.Sprintf(
		"( %d | %d ) :: %v",
		s.Season,
		s.LastEpisodeSeen,
		s.Rating,
	)
}
