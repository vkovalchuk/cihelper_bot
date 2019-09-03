package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
)

type Prompt struct {
	Input        KnownMsg `json:"input"`
	MsgChannelId string   `json:"channel_id"`
	MsgTs        string   `json:"msg_ts"`
	Status       string   `json:"status,omitempty"`
}

type BotData struct {
	Prompts []Prompt `json:"prompts"`
}

func NewStorage() BotData {
	return BotData{}
}

func (d *BotData) FindByBuildId(an_build string) *Prompt {
	for _, pr := range d.Prompts {
		if pr.Input.ProductBuildId == an_build {
			return &pr
		}
	}
	return nil
}

func (d *BotData) FindByBranch(branch string) *Prompt {
	for _, pr := range d.Prompts {
		if pr.Input.Branch == branch {
			return &pr
		}
	}
	return nil
}

func (d *BotData) AddPrompt(input KnownMsg, channelId, msgTs string) {
	pr := Prompt{Input: input, MsgChannelId: channelId, MsgTs: msgTs}
	d.Prompts = append(d.Prompts, pr)
}

func (d *BotData) DeletePrompt(elem *Prompt) {
	for i, pr := range d.Prompts {
		if pr == *elem {
			d.Prompts = append(d.Prompts[:i], d.Prompts[i+1:]...)
			return
		}
	}
}

func (b *Bot) ReadTrackingFile() {
	content, err := ioutil.ReadFile(b.trackFile)
	if err != nil {
		fmt.Println("ERROR reading track file:", b.trackFile)
		return
	}
	err = json.Unmarshal(content, &b.storage)
	if err != nil {
		log.Fatal(err)
	}
}

func (b *Bot) SaveTrackingFile() {
	content, err := json.Marshal(&b.storage)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(b.trackFile, content, 0644)
	if err != nil {
		log.Fatal(err)
	}
}
