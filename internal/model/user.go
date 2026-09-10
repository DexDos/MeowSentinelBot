package model

import (
	"github.com/mymmrac/telego"
)

type User interface {
	GetData() string
	GetDataForNewRecord() (DataForNewStack, error)
	GetTelegramID() int64
	GetTelegramUsername() string
}

type OddData interface{}
type DataForNewStack struct {
	stackID   int64
	recTitle  string
	recType   string
	recStatus string
}
type UserData struct {
	Payload   string
	StackData DataForNewStack
	OddData
}

type TelegramUser struct {
	user     telego.User
	userData UserData
}

func NewTelegramUser(user telego.User, p string) *TelegramUser {
	return &TelegramUser{
		user: user,
		userData: UserData{
			Payload: p,
		},
	}
}

func (u *TelegramUser) GetData() string {
	if u.userData.Payload != "" {
		return u.userData.Payload
	}
	return "null"
}

func (u *TelegramUser) GetDataForNewRecord() (DataForNewStack, error) {
	if u.userData.Payload == "" {
		panic("telegram user has no payload")
	}

}

func (u *TelegramUser) GetTelegramID() int64 {
	return u.user.ID
}

func (u *TelegramUser) GetTelegramUsername() string {
	return u.user.Username
}
