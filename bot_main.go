package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/nlopes/slack"
)

func main() {

	cfg := &BotConfig{}
	if e := cfg.LoadFromFile("botconfig.json"); e != nil {
		fmt.Println("ERROR read json", e)
	} else {
		fmt.Println("Config read: name=", cfg.BotName, "trackfile=", cfg.TrackFile)
	}

	fmt.Println("Starting CI Helper Slack bot")
	rtm := CreateSlackClient(cfg.ApiToken, cfg.BotName)

	build_bot := &Bot{
		rtm:       rtm,
		trackFile: cfg.TrackFile,
		storage:   NewStorage(),
	}
	build_bot.ReadTrackingFile()
	Debug = cfg.DebugRules

	go rtm.ManageConnection()

	SubscribeEndpoints(build_bot)
	go ListenHTTP()

	build_bot.LoopEvents(rtm)

	fmt.Println("Clean stop")
}

func CreateSlackClient(apiKey string, botName string) *slack.RTM {
	client := slack.New(apiKey)

	f, err := os.OpenFile(botName+".log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	// do not f.Close()

	comboLog := io.MultiWriter(os.Stdout, f)
	logger := log.New(comboLog, botName+": ", log.Lshortfile|log.LstdFlags)
	wrapped := &WrappedLogger{Logger: logger, filter1: "Sending PING", filter2: `Incoming Event: {"type":"pong"`}
	slack.SetLogger(wrapped)
	client.SetDebug(true)

	rtm := client.NewRTM()
	return rtm
}

func (b *Bot) LoopEvents(rtm *slack.RTM) {
eventLoop:
	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {

		case *slack.MessageEvent:
			if ev.Msg.Text == "ENOUGH" {
				fmt.Println("Received ENOUGH -- disconnecting")
				rtm.Disconnect()
				break eventLoop
			}
			details, err := RecognizeMessage(ev)
			if err == nil {
				b.React(details, ev.Channel)
			} else if err == ErrNotMatched {
				b.DoCommand(ev.Channel, ev.Msg.Text, ev.User)
			} else {
				fmt.Println("IGNORED", err.Error())
			}

		default:
			if msg.Type != "latency_report" {
				fmt.Println("--- Event Received: ", msg.Type)
			}
		}
	}
}

type WrappedLogger struct {
	*log.Logger
	filter1, filter2 string
}

func (l *WrappedLogger) Fatal(v ...interface{})                 { l.Logger.Fatal(v...) }
func (l *WrappedLogger) Fatalf(format string, v ...interface{}) { l.Logger.Fatalf(format, v...) }
func (l *WrappedLogger) Fatalln(v ...interface{})               { l.Logger.Fatalln(v...) }

func (l *WrappedLogger) Flags() int     { return l.Logger.Flags() }
func (l *WrappedLogger) Prefix() string { return l.Logger.Prefix() }

func (l *WrappedLogger) Output(calldepth int, s string) error {
	if strings.HasPrefix(s, l.filter1) || strings.HasPrefix(s, l.filter2) {
		return nil
	}
	return l.Logger.Output(calldepth, s)
}

func (l *WrappedLogger) Panic(v ...interface{})                 { l.Logger.Panic(v...) }
func (l *WrappedLogger) Panicf(format string, v ...interface{}) { l.Logger.Panicf(format, v...) }
func (l *WrappedLogger) Panicln(v ...interface{})               { l.Logger.Panicln(v...) }
func (l *WrappedLogger) Print(v ...interface{})                 { l.Logger.Print(v...) }
func (l *WrappedLogger) Printf(format string, v ...interface{}) { l.Logger.Printf(format, v...) }
func (l *WrappedLogger) Println(v ...interface{})               { l.Logger.Println(v...) }

func (l *WrappedLogger) SetFlags(flag int)       { l.Logger.SetFlags(flag) }
func (l *WrappedLogger) SetOutput(w io.Writer)   { l.Logger.SetOutput(w) }
func (l *WrappedLogger) SetPrefix(prefix string) { l.Logger.SetPrefix(prefix) }
