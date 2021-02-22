package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var initialNGWords = "戌神ころね,リゼ・ヘルエスタ,Vtuber,VTuber,vtuber,バーチャルユーチューバー,バーチャルYouTuber,笹木咲,戌亥とこ"
var ngWords []string
var adminID string
var mainChannelID string

func main() {
	println(os.Getenv("GO_ENV"))
	err := godotenv.Load(fmt.Sprintf("./%s.env", os.Getenv("GO_ENV")))
	if err != nil {
		log.Println(err)
	}

	Token := os.Getenv("DISCORD_TOKEN")
	adminID = os.Getenv("ADMIN_ID")
	mainChannelID = os.Getenv("MAIN_CHANNEL_ID")
	if Token == "" || adminID == "" || mainChannelID == "" {
		log.Fatalln("No required env.")
		return
	}

	_, err = os.Stat("./data.txt")
	if err != nil {
		os.Create("./data.txt")
		ioutil.WriteFile("./data.txt", []byte(initialNGWords), 0666)
	}

	bytes, err := ioutil.ReadFile("./data.txt")
	if err != nil {
		log.Fatalln(err)
	}

	log.Println(string(bytes))
	ngWords = strings.Split(string(bytes), ",")

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalln("Cannot create Discord session,", err)
		return
	}

	dg.AddHandler(ready)
	dg.AddHandler(generateMessegaCreate)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Cannot connect,", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	s.ChannelMessageSend(mainChannelID, "新しいバージョンがリリースされました👮‍♂️")
	s.UpdateStatus(0, "MAKE CHINA GREAT")
}

func generateMessegaCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	ngReg, _ := regexp.Compile("^!ng ")
	rmngReg, _ := regexp.Compile("^!rmng ")
	showReg, _ := regexp.Compile("^!showng")

	if showReg.MatchString(m.Content) {
		str := ""
		for i, w := range ngWords {
			if i != 0 {
				str += ", "
			}
			str += w
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("現在設定されているNGワードは\n```\n%s\n```です", str))
	}

	if strings.Contains(m.Content, "youtube.com") || strings.Contains(m.Content, "youtu.be") {
		html, err := getHTMLStr(m.Content)
		if err != nil {
			log.Println(err)
			return
		}
		if containsNGWords(html) {
			s.ChannelMessageSend(m.ChannelID, m.Author.Mention()+" ピピーッ！👮‍♂️バーチャルYouTuberを検出しました！削除します！🙅‍♂️🙅‍♂️🙅‍♂️")
			s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
		}
	}

	if ngReg.MatchString(m.Content) {
		if m.Author.ID != adminID {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️権限がありません！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		var str string
		add := ""
		fmt.Sscanf(m.Content, "!ng %s %s", &str, &add)
		if strings.Contains(str, ",") || add != "" {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️フォーマット違反です！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		if containNG(str) {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️既に追加されているNGワードです！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		err := addNG(str)
		if err != nil {
			log.Fatal(err)
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s をNGワードに追加しました", str))
		err = loadNG()
		if err != nil {
			log.Fatal(err)
		}
	}

	if rmngReg.MatchString(m.Content) {
		if m.Author.ID != adminID {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️権限がありません！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		var str string
		add := ""
		fmt.Sscanf(m.Content, "!rmng %s %s", &str, &add)
		if strings.Contains(str, ",") || add != "" {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️フォーマット違反です！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		if !containNG(str) {
			s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️存在しないNGワードです！🙅‍♂️🙅‍♂️🙅‍♂️")
			return
		}
		err := removeNG(str)
		if err != nil {
			log.Fatal(err)
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s をNGワードから削除しました", str))
		err = loadNG()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func containNG(word string) bool {
	for _, w := range ngWords {
		if w == word {
			return true
		}
	}
	return false
}

func getHTMLStr(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	buf := bytes.NewBuffer(body)
	html := buf.String()
	return html, nil
}

func containsNGWords(str string) bool {
	for _, ng := range ngWords {
		if strings.Contains(str, ng) {
			return true
		}
	}
	return false
}

func addNG(str string) error {
	f, err := os.OpenFile("./data.txt", os.O_APPEND|os.O_WRONLY, 0600)
	defer f.Close()

	fmt.Fprint(f, ","+str)
	return err
}

func removeNG(word string) error {
	result := []string{}
	str := ""
	for _, v := range ngWords {
		if v != word {
			result = append(result, v)
		}
	}
	for i, w := range result {
		if i != 0 {
			str += ","
		}
		str += w
	}
	err := ioutil.WriteFile("./data.txt", []byte(str), 0666)
	return err
}

func loadNG() error {
	bytes, err := ioutil.ReadFile("./data.txt")
	ngWords = strings.Split(string(bytes), ",")
	return err
}
