package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type BotConfig struct {
	BotName      string `json:"bot_name,omitempty"`
	ApiToken     string `json:"api_token"`
	ListenUserId string `json:"listen_userid,omitempty"`
	TrackFile    string `json:"trackfile"`
	DebugRules   bool   `json:"debug_rules"`
}

func (cfg *BotConfig) LoadFromFile(path string) error {
	file, e := ioutil.ReadFile(path)
	if e != nil {
		return e
	}

	e = json.Unmarshal(file, cfg)
	return e
}

func test() {
	cfg := &BotConfig{}
	if e := cfg.LoadFromFile("botconfig.json"); e != nil {
		fmt.Println("ERROR read json", e)
	} else {
		fmt.Println("Config read: name=", cfg.BotName, "trackfile=", cfg.TrackFile)
	}
}
