package model

import "time"

type Invite struct {
	ID        int64
	Code      string
	StackID   int64
	CreatedBy int64
	ExpiresAt time.Time
	IsUsed    bool
	CreatedAt time.Time
}

func (i Invite) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}
