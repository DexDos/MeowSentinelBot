package model

import (
	"MeowSentinelBot/internal/utils"
	"errors"

	"github.com/mymmrac/telego"
)

type User interface {
	GetPayload() string
	GetCurrentStackID() int64
	SetCurrentStackID(int64)
	GetDataForNewRecord() (DataForNewStack, error)
	GetTelegramID() int64
	GetTelegramUsername() string
}

type OddData interface{}
type DataForNewStack struct {
	StackID         uint64
	RecTitle        string
	RecType         string
	RecLastSeason   uint8
	RecStartEpisode uint16
}
type UserData struct {
	Payload        string
	CurrentStackID int64
	StackData      DataForNewStack
	OddData
}

type TelegramUser struct {
	User     *telego.User
	UserData UserData
}

func (u *TelegramUser) GetCurrentStackID() int64 {
	return u.UserData.CurrentStackID
}

func (u *TelegramUser) SetCurrentStackID(newID int64) {
	u.UserData.CurrentStackID = newID
}

func NewTelegramUser(user *telego.User, payload string) *TelegramUser {
	return &TelegramUser{
		User: user,
		UserData: UserData{
			Payload: payload,
		},
	}
}

func (u *TelegramUser) GetPayload() string {
	if u.UserData.Payload != "" {
		return u.UserData.Payload
	}
	return "null"
}
func (u *TelegramUser) SetPayload(p string) error {
	if p == "" {
		return errors.New("payload is empty")
	}
	u.UserData.Payload = p
	return nil
}

func (u *TelegramUser) GetDataForNewRecord() (DataForNewStack, error) {
	if u.UserData.Payload == "" {
		return DataForNewStack{}, errors.New("user's payload is empty")
	}
	var data DataForNewStack
	data.StackID = uint64(u.UserData.CurrentStackID)

	err := utils.ParseJSONFromString(u.UserData.Payload, &data)
	return data, err
}

func (u *TelegramUser) GetTelegramID() int64 {
	return u.User.ID
}

func (u *TelegramUser) GetTelegramUsername() string {
	return u.User.Username
}
