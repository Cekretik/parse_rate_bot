package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Instrument struct {
	//OMSId        int     `json:"OMSId"`
	InstrumentId int     `json:"InstrumentId"`
	LastTradedPx float64 `json:"LastTradedPx"`
}

var lastMessageID int

func calculateRateWithCommission(rate float64, commission float64) float64 {
	withdrawalFee := 0.26
	totalCommission := commission + withdrawalFee
	adjustedRate := rate * (1 - totalCommission/100)
	return math.Round(adjustedRate*100) / 100
}
func calculateRateWithInternCommission(rate float64, commission float64) float64 {
	adjustedRate := rate * (1 - commission/100)
	return math.Round(adjustedRate*100) / 100
}

func getUSDTPrice() (float64, error) {
	url := "https://apexapi.bitazza.com:8443/AP/GetLevel1Summary?OMSId=1"
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("недоступно: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var rawData []string
	err = json.Unmarshal(body, &rawData)
	if err != nil {
		return 0, err
	}

	for _, item := range rawData {
		var instrument Instrument
		if err := json.Unmarshal([]byte(item), &instrument); err != nil {
			continue
		}
		if instrument.InstrumentId == 5 {
			return instrument.LastTradedPx, nil
		}
	}

	return 0, fmt.Errorf("не удалось найти USDT/THB")
}
func MenuButtonKeyboard() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Меню"),
		),
	)
}

func startHandler(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Выберите интересующую комиссию для курса:")
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("1.5%", "1.5"),
			tgbotapi.NewInlineKeyboardButtonData("2.0%", "2.0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("2.5%", "2.5"),
			tgbotapi.NewInlineKeyboardButtonData("3.0%", "3.0"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Себестоимость с учётом комиссии", "0.26"),
		),
	)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func callbackHandler(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		commission, err := strconv.ParseFloat(update.CallbackQuery.Data, 64)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка обработки комиссии"))
			return
		}

		rate, err := getUSDTPrice()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Ошибка получения курса"))
			return
		}

		var finalRate float64
		var response string

		if commission == 0.26 {
			finalRate = calculateRateWithInternCommission(rate, commission)
			response = fmt.Sprintf("Курс себестоимости с комиссией %.2f%%: %.2f", commission, finalRate)
		} else {
			finalRate = calculateRateWithCommission(rate, commission)
			response = fmt.Sprintf("Курс с комиссией %.2f%%: %.2f", commission, finalRate)
		}

		if lastMessageID != 0 {
			deleteMsg := tgbotapi.NewDeleteMessage(update.CallbackQuery.Message.Chat.ID, lastMessageID)
			bot.Send(deleteMsg)
		}

		newMsg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, response)
		sentMsg, _ := bot.Send(newMsg)

		lastMessageID = sentMsg.MessageID
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil && update.Message.Text == "/start" {
			startHandler(bot, update)
		} else if update.CallbackQuery != nil {
			callbackHandler(bot, update)
		}
	}
}
