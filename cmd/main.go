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

	bot.StartBot()
}
