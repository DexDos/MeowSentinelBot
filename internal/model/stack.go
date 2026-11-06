package model

import "time"

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

type StackInfo struct {
	ID           int64
	OwnerID      int64
	Title        string
	Role         string
	EntriesCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (s StackInfo) IsOwner(userID int64) bool {
	return s.OwnerID == userID || s.Role == RoleOwner
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
