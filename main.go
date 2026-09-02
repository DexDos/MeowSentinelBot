package main

import (
	"context"
	"fmt"
	"github.com/mymmrac/telego"
	"os"
)

func main() {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	bot, err := telego.NewBot(botToken, telego.WithDefaultDebugLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	botUser, err := bot.GetMe(context.Background())
	if err != nil {
		fmt.Println("Error: ", err)
	}

	fmt.Printf("Bot user: %+v\n", botUser)
}
