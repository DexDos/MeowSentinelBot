package model

import (
	"MeowSentinelBot/internal/utils"
	"errors"

	"github.com/mymmrac/telego"
)

type User interface {
	GetData() string
	GetCurrentStackID() int64
	SetCurrentStackID(int64)
	GetDataForNewRecord() (DataForNewStack, error)
	GetTelegramID() int64
	GetTelegramUsername() string
}

type OddData interface{}
type DataForNewStack struct {
	StackID   int64  `json:"stack_id"`
	RecTitle  string `json:"rec_title"`
	RecType   string `json:"rec_type"`
	RecStatus string `json:"rec_status"`
}
type UserData struct {
	Payload        string
	CurrentStackID int64
	StackData      DataForNewStack
	OddData
}

type TelegramUser struct {
	user     telego.User
	userData UserData
}

func (u *TelegramUser) GetCurrentStackID() int64 {
	return u.userData.CurrentStackID
}

func (u *TelegramUser) SetCurrentStackID(newID int64) {
	u.userData.CurrentStackID = newID
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
		return DataForNewStack{}, errors.New("user's payload is empty")
	}
	var data DataForNewStack
	data.StackID = u.userData.CurrentStackID

	err := utils.ParseJSONFromString(u.userData.Payload, &data)
	return data, err
}

func (u *TelegramUser) GetTelegramID() int64 {
	return u.user.ID
}

func (u *TelegramUser) GetTelegramUsername() string {
	return u.user.Username
}
