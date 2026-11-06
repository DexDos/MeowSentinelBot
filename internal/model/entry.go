package model

import (
	"fmt"
	"html"
	"time"
)

type Entry struct {
	ID          int64
	StackID     int64
	Title       string
	Type        string
	Season      int64
	Episode     int64
	Rating      Score
	RatingCount int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e Entry) FormattedLine() string {
	escapedTitle := html.EscapeString(e.Title)
	return fmt.Sprintf(
		"<b>#%d</b> %s (S: %d | Ep: %d) %s\n",
		e.ID,
		escapedTitle,
		e.Season,
		e.Episode,
		e.Rating.Stars(),
	)
}
