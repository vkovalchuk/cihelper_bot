package main

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
)

type Bot struct {
	rtm          *slack.RTM
	trackFile    string
	storage      BotData
	cluster_busy string
}

const BD_SLACK_BOTID_ANSIBLE = "B8BPFRXUL"
const BD_SLACK_BOTID_JENKINS = "B8QMGB0P4"
const BD_SLACK_BOTID_BIGDATA = "U9ZLH3A7L"

var Debug bool = false

// Union of all known message types
type KnownMsg struct {
	MsgType          string `json:"event_type"`
	ProductBuildId   string `json:"an_build"`
	Branch           string `json:"branch,omitempty"`
	Outcome          string `json:"rt_outcome,omitempty"`
}

const msgTypeNewBuild = "new_jenkins_build"
const msgRegxNewBuild = `<http://jenkins.devops.company.com/job/product\-build/([0-9]+)/\|` +
	`Build product\-build # ([0-9]+)>, branch ([a-zA-Z0-9_\.\-/]+): (.+)`

const msgTypeRTOutcome = "reg_tests_outcome"
const msgRegxRTOutcome = `<http://jenkins.devops.company.com/job/run\-regression\-tests/([0-9]+)/\|` +
	`Regression tests #([0-9]+)> for build ([a-zA-Z0-9_\.\-/]+) finished, outcome: (.+)`

const msgTypeRTStarted = "reg_tests_started"
const msgRegxRTStarted = `Playbook regtests2_job.yaml \*STARTED\* at ([0-9\.]+)`
const msgRegxRTStartedLine3 = `Product build: ([a-zA-Z0-9_\.\-/]+)`

const msgTypeRTDone = "reg_tests_done"
const msgRegxRTDone = `Playbook regtests2_job.yaml \*DONE\* at ([0-9\.]+)`
const msgRegxRTDoneLine2 = `Product build: ([a-zA-Z0-9_\.\-/]+)`

var ErrNotMatched = errors.New("Message doesn't match any regexp")

// Determines msgType and KnownMsg params of a Slack message text
var msgRules = []struct {
	msgType         string
	sender          string
	msgRegexp       *regexp.Regexp
	msgRegexpParams []string
	checkParams     func(lines, params []string) (KnownMsg, error)
}{
	{
		msgType:         msgTypeNewBuild,
		sender:          BD_SLACK_BOTID_JENKINS,
		msgRegexp:       regexp.MustCompile(msgRegxNewBuild),
		msgRegexpParams: []string{"FULL", "url_build_num", "msg_build_num", "REFTO", "build_result"},
		checkParams:     checkParamsNewBuild,
	},
	{
		msgType:         msgTypeRTOutcome,
		sender:          BD_SLACK_BOTID_JENKINS,
		msgRegexp:       regexp.MustCompile(msgRegxRTOutcome),
		msgRegexpParams: []string{"FULL", "url_build_num", "msg_build_num", "an_build", "rt_outcome"},
		checkParams:     checkParamsRTOutcome,
	},
	{
		msgType:         msgTypeRTStarted,
		sender:          BD_SLACK_BOTID_ANSIBLE,
		msgRegexp:       regexp.MustCompile(msgRegxRTStarted),
		msgRegexpParams: []string{"FULL", "hostname"},
		checkParams:     checkParamsRTStarted,
	},
	{
		msgType:         msgTypeRTDone,
		sender:          BD_SLACK_BOTID_ANSIBLE,
		msgRegexp:       regexp.MustCompile(msgRegxRTDone),
		msgRegexpParams: []string{"FULL", "hostname"},
		checkParams:     checkParamsRTDone,
	},
}

func checkParamsNewBuild(lines, params []string) (d KnownMsg, err error) {
	an_build := strings.TrimSpace(lines[1])
	if strings.HasPrefix(an_build, "release") {
		return d, errors.New("Release branch, Jenkins will trigger regTest")
	}
	d.ProductBuildId = an_build
	// params: [FULL, url_build_num, msg_build_num, REFTO, build_result]
	if build_result := params[4]; build_result != "SUCCESS" {
		return d, errors.New("Failed build, regTest not needed")
	}
	d.Branch = params[3]
	return
}

func checkParamsRTOutcome(lines, params []string) (d KnownMsg, err error) {
	d.ProductBuildId = params[3]
	d.Outcome = params[4]
	return
}

func checkParamsRTStarted(lines, params []string) (d KnownMsg, err error) {
	line3 := strings.TrimSpace(lines[2])
	line3params := regexp.MustCompile(msgRegxRTStartedLine3).FindStringSubmatch(line3)
	if line3params == nil {
		return d, errors.New("Unexpected line3 of reg_tests_started message")
	}
	d.ProductBuildId = line3params[1]
	return
}

func checkParamsRTDone(lines, params []string) (d KnownMsg, err error) {
	line2 := strings.TrimSpace(lines[1])
	line2params := regexp.MustCompile(msgRegxRTDoneLine2).FindStringSubmatch(line2)
	if line2params == nil {
		return d, errors.New("Unexpected line2 of reg_tests_done message")
	}
	d.ProductBuildId = line2params[1]
	return
}

func RecognizeMessage(ev *slack.MessageEvent) (d KnownMsg, err error) {
	lines := strings.Split(ev.Msg.Text, "\n")
	if Debug {
		fmt.Println("SEARCH user=", ev.User, "bot:", ev.BotID, "line0=", lines[0])
	}
	for _, r := range msgRules {
		if r.sender == ev.User || r.sender == ev.BotID {
			params := r.msgRegexp.FindStringSubmatch(lines[0])
			if params != nil {
				d, err = r.checkParams(lines, params)
				if err == nil {
					d.MsgType = r.msgType
					if Debug {
						fmt.Println("Matched and suitable")
					}
				}
				return
			}
		}
	}

	if Debug {
		fmt.Println("Not parsed")
	}
	return d, ErrNotMatched
}

func (b *Bot) React(input KnownMsg, сhannelId string) {
	switch input.MsgType {
	case msgTypeNewBuild:
		b.reactOnNewBuild(input, сhannelId)
	case msgTypeRTStarted:
		b.reactOnRegTestStart(input)
	case msgTypeRTDone:
		b.reactOnRegTestFinish(input)
	case msgTypeRTOutcome:
		b.reactOnRTOutcome(input)
	default:
		fmt.Println("ERROR!!! Cannot handle:", input, "in", сhannelId)
	}
}

// Delete previous prompt for the same branch
// Add prompt for new build
// Save tracking
func (b *Bot) reactOnNewBuild(input KnownMsg, channelId string) {
	pr := b.storage.FindByBranch(input.Branch)
	if pr != nil {
		b.rtm.DeleteMessage(channelId, pr.MsgTs)
		b.storage.DeletePrompt(pr)
	}

	// Now react
	an_build := input.ProductBuildId
	msgText, msgParams := preparePromptMessage(an_build)
	if b.cluster_busy != "" {
		msgText = MsgTextClusterBusy + b.cluster_busy
		msgParams.Attachments = nil
	}
	postedChanId, reactTimestamp, err := b.rtm.PostMessage(channelId, msgText, msgParams)
	if err != nil {
		fmt.Println("ERROR Reacting", err, "in", postedChanId)
	}
	fmt.Println("REACTED! ts: ", reactTimestamp, " -> ", an_build)

	b.storage.AddPrompt(input, channelId, reactTimestamp)
	b.SaveTrackingFile()
}

func preparePromptMessage(an_build string) (reactText string, reactParams slack.PostMessageParameters) {

	reactText = "Product build " + an_build + " is created"

	runRegtestUrl := "http://jenkins.devops.company.com/job/run-regression-tests/buildWithParameters?" +
		"token=12345678&AN_BUILD=" + an_build + "&TESTSET=core"
	discardUrl := "http://10.77.2.4:8000/buildbot?op=disableBuild&AN_BUILD=" + an_build

	attachment := slack.Attachment{
		Text:       "Verifying new build will take 90 minutes and block DEV cluster",
		CallbackID: "an_build_reaction",
		Actions: []slack.AttachmentAction{
			slack.AttachmentAction{
				Name:  "btnRunRegTests",
				Value: an_build,
				Text:  "Run Regression Test",
				Type:  "button",
				Style: "danger",
				URL:   runRegtestUrl,
			},
			slack.AttachmentAction{
				Name:  "btnDisableBuild",
				Value: an_build,
				Text:  "Ignore this build",
				Type:  "button",
				URL:   discardUrl,
			},
		},
	}

	reactParams = slack.PostMessageParameters{
		EscapeText:  false,
		AsUser:      true,
		Username:    BD_SLACK_BOTID_BIGDATA,
		Attachments: []slack.Attachment{attachment},
	}
	return
}

func (b *Bot) reactOnRegTestStart(input KnownMsg) {
	// Disable all prompts
	b.cluster_busy = input.ProductBuildId
	newText := MsgTextClusterBusy + input.ProductBuildId
	for _, pr := range b.storage.Prompts {
		_, newTs, _, err := b.rtm.Client.SendMessage(pr.MsgChannelId,
			slack.MsgOptionText(newText, false),
			slack.MsgOptionUpdate(pr.MsgTs),
			slack.MsgOptionAttachments(slack.Attachment{}))
		if err != nil {
			fmt.Println("ERROR disabling msg", pr, ":", err.Error())
		}
		pr.MsgTs = newTs
	}
}

func (b *Bot) reactOnRegTestFinish(input KnownMsg) {
	b.cluster_busy = ""
	// Enable prompts
	for _, pr := range b.storage.Prompts {
		an_build := pr.Input.ProductBuildId
		newText, newParams := preparePromptMessage(an_build)
		if pr.Status == "disabled" {
			newText = MsgTextBuildDisabled + an_build
			newParams.Attachments = nil
		}
		err := b.sendUpdate(&pr, newText, newParams.Attachments)
		if err != nil {
			fmt.Println("ERROR restoring msg", pr, ":", err.Error())
		}
	}
}

func (b *Bot) reactOnRTOutcome(input KnownMsg) {
	// Delete prompt on SUCCESS, maybe update on FAILURE
	if input.Outcome != "SUCCESS" {
		return
	}
	an_build := input.ProductBuildId
	pr := b.storage.FindByBuildId(an_build)
	if pr == nil {
		fmt.Println("WARNING: Cannot find prompt for", an_build)
		return
	}

	fmt.Println("Regression on build SUCCESS, delete", pr)
	newText := "Product build " + an_build + " was verified by regression tests"
	err := b.sendUpdate(pr, newText, []slack.Attachment{})
	if err != nil {
		fmt.Println("ERROR deleting ", pr, ":", err.Error())
	}
	pr.Status = "verified"
	b.storage.DeletePrompt(pr)
	b.SaveTrackingFile()
}

func (b *Bot) sendUpdate(pr *Prompt, newText string, newAtt []slack.Attachment) (err error) {
	_, newTs, _, err := b.rtm.Client.SendMessage(pr.MsgChannelId,
		slack.MsgOptionText(newText, false),
		slack.MsgOptionUpdate(pr.MsgTs),
		slack.MsgOptionAttachments(newAtt...))
	if newTs != "" {
		pr.MsgTs = newTs
	}
	return
}

func (b *Bot) DoCommand(channelId, msgText, fromUserId string) {
	if msgText == "list" {
		report := "Known prompts (an_build : message_ts):"
		for _, pr := range b.storage.Prompts {
			report += "\n" + pr.Input.ProductBuildId + " : " + pr.MsgTs
		}
		b.rtm.PostEphemeral(channelId, fromUserId, slack.MsgOptionText(report, false))
	} else if msgText == "clear" {
		for _, pr := range b.storage.Prompts {
			b.rtm.DeleteMessage(pr.MsgChannelId, pr.MsgTs)
		}
		b.storage = NewStorage()
		b.SaveTrackingFile()
	} else {
		if Debug {
			fmt.Println("Not matched and not a command", msgText)
		}
	}
}

const MsgTextBuildDisabled = "User has decided to skip regression test on "

const MsgTextClusterBusy = "DEV cluster is busy: regtest of "

func (b *Bot) disableBuild(an_build string) error {
	pr := b.storage.FindByBuildId(an_build)
	if pr == nil {
		return errors.New("No such build: " + an_build)
	}

	msgTs := pr.MsgTs
	newText := MsgTextBuildDisabled + an_build
	_, newTs, _, err := b.rtm.Client.SendMessage(pr.MsgChannelId,
		slack.MsgOptionText(newText, false),
		slack.MsgOptionUpdate(msgTs),
		slack.MsgOptionAttachments(slack.Attachment{}))
	fmt.Println("Sent update, new=", newTs, "prev=", msgTs)
	if newTs != msgTs {
		pr.MsgTs = newTs
		pr.Status = "disabled"
		b.SaveTrackingFile()
	}

	return err
}
