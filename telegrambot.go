package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"

	"github.com/Sirupsen/logrus"
)

const (
	URL             = "https://api.telegram.org/bot"
	IndiaTimeZoneID = "Asia/Kolkata"
)

type BotInfo struct {
	URL        *url.URL
	WebhookURL *url.URL
}

var TOKEN string
var WEBHOOK_URL string
var NAMES_FILE string
var NOPINGDAYS string
var HOUR int
var MINUTE int

func readAllEnvs() {
	TOKEN := os.Getenv("TOKEN")
	if TOKEN== "" {
		return fmt.Errorf("Please set environment variable TOKEN")
	}

	WEBHOOK_URL := os.Getenv("WEBHOOK_URL")
	if WEBHOOK_URL == "" {
		return fmt.Errorf("Please set environment variable WEBHOOK_URL")
	}

	NAMES_FILE := os.Getenv("NAMES")
	if NAMES_FILE == "" {
		NAMES_FILE = "names.yml"
		logrus.Infof("Using default names file name: %s", NAMES_FILE)
	}

	NOPINGDAYS := os.Getenv("NOPINGDAYS")
	if NOPINGDAYS == "" {
		NOPINGDAYS = "Saturday,Sunday"
		logrus.Infof("NOPINGDAYS not set using default: %s", NOPINGDAYS)
	}

	// set HOUR
	hourstr := os.Getenv("HOUR")
	HOUR, err := strconv.Atoi(hourstr)
	if err != nil {
		HOUR = 12
		logrus.Infof("Using default hour: %d", HOUR)
	}

	// set minute
	minstr := os.Getenv("MINUTE")
	MINUTE, err := strconv.Atoi(minstr)
	if err != nil {
		MINUTE = 45
		logrus.Infof("Using default minutes: %d", MINUTE)
	}
}

func (b *BotInfo) InitBotObject() error {
	var err error
	b.URL, err = url.Parse(URL + TOKEN)
	if err != nil {
		return fmt.Errorf("Error parsing url: %s, Error: %v", (URL + TOKEN), err)
	}

	b.WebhookURL, err = url.Parse(WEBHOOK_URL)
	if err != nil {
		return fmt.Errorf("Error parsing url: %s, Error: %v", WEBHOOK_URL, err)
	}
	b.WebhookURL.Path = path.Join(b.WebhookURL.Path, TOKEN)

	logrus.Infof("Botinfo object initialized.")
	return nil
}

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	UserName  string `json:"username,omitempty"`
}

type Chat struct {
	ID                          int    `json:"id"`
	Type                        string `json:"type"`
	Title                       string `json:"title,omitempty"`
	Username                    string `json:"username,omitempty"`
	FirstName                   string `json:"first_name,omitempty"`
	LastName                    string `json:"last_name,omitempty"`
	AllMembersAreAdministrators bool
}

type MessageEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
	URL    string `json:"url,omitempty"`
	User   User   `json:"user,omitempty"`
}

type Message struct {
	MessageID int             `json:"message_id"`
	From      User            `json:"from,omitempty"`
	Date      int             `json:"date"`
	Chat      Chat            `json:"chat"`
	Text      string          `json: "text,omitempty"`
	Entities  []MessageEntity `json:"entities,omitempty"`
}

type Update struct {
	UpdateID int     `json:"update_id"`
	Message  Message `json:"message"`
}

type Response struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

type ResponseSentMessage struct {
	OK          bool    `json:"ok"`
	Result      Message `json:"result,omitempty"`
	ErrorCode   int     `json:"error_code,omitempty"`
	Description string  `json:"description,omitempty"`
}

func (b *BotInfo) GetUpdates() ([]Update, error) {
	updateURL, err := url.Parse(b.URL.String())
	if err != nil {
		return []Update{}, fmt.Errorf("Error parsing url: %s, Error: %v", b.URL.String(), err)
	}
	updateURL.Path = path.Join(updateURL.Path, "getUpdates")

	resp, err := http.Get(updateURL.String())
	if err != nil {
		return []Update{}, fmt.Errorf("Error getting latest updates. Error: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Update{}, fmt.Errorf("Error reading response body. Error: %v", err)
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return []Update{}, fmt.Errorf("Could not unmarshal the response. Error: %v", err)
	}
	if response.OK {
		return response.Result, nil
	} else {
		return []Update{}, fmt.Errorf("No data received")
	}
}

func (b *BotInfo) SendMessage(chatid int, message string) error {
	sendMsgURL, err := url.Parse(b.URL.String())
	if err != nil {
		return fmt.Errorf("Error parsing url: %s, Error: %v", b.URL.String(), err)
	}
	sendMsgURL.Path = path.Join(sendMsgURL.Path, "sendMessage")

	q := sendMsgURL.Query()
	q.Add("chat_id", strconv.Itoa(chatid))
	q.Add("text", message)
	sendMsgURL.RawQuery = q.Encode()

	resp, err := http.Get(sendMsgURL.String())
	if err != nil {
		return fmt.Errorf("Could not post request. Error: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Error reading response body. Error: %v", err)
	}

	var response ResponseSentMessage
	err = json.Unmarshal(body, &response)
	if err != nil {
		return fmt.Errorf("Could not unmarshal the response. Error: %v", err)
	}
	if response.OK {
		logrus.Infof("Request to server was made")
		return nil
	} else {
		return fmt.Errorf("No data received. Error Code: %d, Desc: %s", response.ErrorCode, response.Description)
	}
}

func PostMessage(message string) {
	var b BotInfo
	err := b.InitBotObject()
	if err != nil {
		logrus.Fatalf("%v", err)
	}

	logrus.Infof("Requesting for updates.")
	updates, err := b.GetUpdates()
	if err != nil {
		logrus.Warningf("Could not get updates. Error: %v", err)
		return
	}

	lastUpdate := updates[len(updates)-1]
	chatid := lastUpdate.Message.Chat.ID

	logrus.Infof("Sending message: %s", message)
	err = b.SendMessage(chatid, message)
	if err != nil {
		logrus.Warningf("Failed sending message. Error: %v", err)
	}
}

func getNames() ([]string, error) {
	type NamesList struct {
		Names []string `yaml:"names"`
	}
	nameFileContents, err := ioutil.ReadFile(NAMES_FILE)
	if err != nil {
		return []string{}, fmt.Errorf("Error while reading names file: %v", err)
	}

	var readNames NamesList
	err = yaml.Unmarshal(nameFileContents, &readNames)
	if err != nil {
		return []string{}, fmt.Errorf("Error while unmarshalling yaml: %v", err)
	}

	return readNames.Names, nil
}

func shouldIPingToday(t time.Time) bool {
	noPingDays := strings.Split(NOPINGDAYS, ",")
	for _, day := range noPingDays {
		if t.Weekday().String() == day {
			return false
		}
	}
	return true
}

func PingForLunch() {
	indiaTZ, err := time.LoadLocation(IndiaTimeZoneID)
	if err != nil {
		logrus.Fatalf("Error parsing timezone. Error: %v", err)
	}

	// flag that keeps track if ping was done or not
	var pingdone bool

	for {
		t := time.Now()
		indiaTime := t.In(indiaTZ)

		if !shouldIPingToday(indiaTime) {
			continue
		}

		// ping only when time matches and if ping not done for that day
		if indiaTime.Hour() == HOUR && indiaTime.Minute() == MINUTE && !pingdone {
			// trigger send message from here:set
			names, err := getNames()
			if err != nil {
				logrus.Errorln(err)
				continue
			}

			var nameTags []string
			for _, name := range names {
				name = fmt.Sprintf("@%s", name)
				nameTags = append(nameTags, name)
			}

			message := fmt.Sprintf("ping for lunch %s", strings.Join(nameTags, " "))
			PostMessage(message)
			pingdone = true
		} else if indiaTime.Hour() == 1 && pingdone {
			// reset pingdone flag at 1 in morning
			pingdone = false
		}
	}
}

func main() {
	// go PingForLunch()
	PingForLunch()
}
