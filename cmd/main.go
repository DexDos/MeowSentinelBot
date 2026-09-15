package main

import (
	config2 "MeowSentinelBot/internal/config"
	"MeowSentinelBot/internal/service"
)

func main() {

	config := config2.MustNewConfig("configs/config.json")
	l := service.NewStdLogger()

	var bot service.BotService
	bot = service.NewBotServiceStd(l, &config.Bot)

	var handler service.APIHandler
	handler = service.NewAPIBotHandler(l, bot)

	var responder service.Responder
	responder = service.NewStdResponder()

	handler.StartBot(&responder)
}
