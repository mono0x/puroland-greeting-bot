package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/lestrrat/go-server-starter/listener"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/mono0x/puroland-greeting-bot/lib"
)

type Action interface {
	Execute() ([]linebot.Message, error)
}

type todayScheduleAction struct {
}

func (a *todayScheduleAction) Execute() ([]linebot.Message, error) {
	today := time.Now()
	api := purobot.NewAPIClient()
	allGreetings, err := api.GetSchedule(today)
	if err != nil {
		switch err {
		case purobot.NotFoundError:
			return []linebot.Message{
				linebot.NewTextMessage("まだ公開されていないよ"),
			}, nil
		case purobot.TemporaryError:
			return []linebot.Message{
				linebot.NewTextMessage("サーバーの調子が悪いみたい"),
			}, nil
		}
		return nil, err
	}

	var greetings []purobot.Greeting
	for _, greeting := range allGreetings {
		if greeting.Deleted {
			continue
		}
		greetings = append(greetings, greeting)
	}

	sort.Slice(greetings, func(i, j int) bool {
		if compared := strings.Compare(greetings[i].EndAt, greetings[j].EndAt); compared != 0 {
			return compared < 0
		}
		if compared := strings.Compare(greetings[i].StartAt, greetings[j].StartAt); compared != 0 {
			return compared < 0
		}
		if compared := strings.Compare(greetings[i].Place.Name, greetings[j].Place.Name); compared != 0 {
			return compared < 0
		}
		return false
	})

	var text string
	text += fmt.Sprintf("%s の予定\n", today.Format("01/02"))
	for i, greeting := range greetings {
		if i != 0 {
			text += "\n\n"
		}

		startAt, err := time.Parse("2006-01-02T15:04:05.999-07:00", greeting.StartAt)
		if err != nil {
			return nil, err
		}

		endAt, err := time.Parse("2006-01-02T15:04:05.999-07:00", greeting.EndAt)
		if err != nil {
			return nil, err
		}

		text += fmt.Sprintf("%s-%s %s\n", startAt.Format("15:04"), endAt.Format("15:04"), greeting.Place.Name)
		for i, character := range greeting.Characters {
			if i != 0 {
				text += "\n"
			}
			text += "- " + character.Name
		}
	}

	return []linebot.Message{
		linebot.NewTextMessage(text),
	}, nil
}

type unimplementedAction struct {
}

func (a *unimplementedAction) Execute() ([]linebot.Message, error) {
	return []linebot.Message{
		linebot.NewTextMessage("未対応だよ"),
	}, nil
}

func createActionFromEvent(event linebot.Event) (Action, error) {
	if event.Type == linebot.EventTypeMessage {
		switch message := event.Message.(type) {
		case *linebot.TextMessage:
			switch message.Text {
			case "今日の予定":
				return &todayScheduleAction{}, nil
			case "翌日のキャラクター":
				return &unimplementedAction{}, nil
			}
		}
		return &unimplementedAction{}, nil
	}
	return nil, nil
}

func run() error {
	_ = godotenv.Load()

	bot, err := linebot.New(os.Getenv("CHANNEL_SECRET"), os.Getenv("CHANNEL_ACCESS_TOKEN"))
	if err != nil {
		return err
	}

	listeners, err := listener.ListenAll()
	if err != nil {
		return err
	}

	var l net.Listener
	if len(listeners) > 0 {
		l = listeners[0]
	} else {
		l, err = net.Listen("tcp", ":14000")
		if err != nil {
			return err
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/line/webhook", func(w http.ResponseWriter, r *http.Request) {
		events, err := bot.ParseRequest(r)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}

		for _, event := range events {
			action, err := createActionFromEvent(event)
			if err != nil {
				log.Println(err)
				return
			}
			if action == nil {
				return
			}
			messages, err := action.Execute()
			if err != nil {
				log.Println(err)
				return
			}
			if _, err := bot.ReplyMessage(event.ReplyToken, messages...).Do(); err != nil {
				log.Println(err)
			}
		}
	})

	server := http.Server{Handler: mux}

	go func() {
		if err := server.Serve(l); err != nil {
			log.Fatal(err)
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)

	for {
		s := <-signalChan
		if s == syscall.SIGTERM {
			if err := server.Shutdown(context.Background()); err != nil {
				return err
			}
			return nil
		}
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
