package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var ngWords = [...]string{
	"戌神ころね", "リゼ・ヘルエスタ", "Vtuber", "VTuber", "vtuber", "バーチャルユーチューバー",
	"バーチャルYouTuber", "笹木咲", "戌亥とこ",
}

func main() {
	println(os.Getenv("GO_ENV"))
	err := godotenv.Load(fmt.Sprintf("./%s.env", os.Getenv("GO_ENV")))
	if err != nil {
		log.Println(err)
	}

	Token := os.Getenv("DISCORD_TOKEN")
	if Token == "" {
		log.Fatalln("No env.")
		return
	}
	log.Println("Token: ", Token)

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalln("Cannot create Discord session,", err)
		return
	}

	dg.AddHandler(ready)
	dg.AddHandler(generateMessegaCreate())
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Cannot connect,", err)
		return
	}

	log.Println("Connected.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	s.UpdateStatus(0, "MAKE CHINA GREAT")
}

func generateMessegaCreate() func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {

		if m.Author.ID == s.State.User.ID {
			return
		}

		if strings.Contains(m.Content, "youtube.com") {
			html, err := getHTMLStr(m.Content)
			if err != nil {
				return
			}
			if containsNGWords(html) {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️バーチャルYouTuberを検出しました！削除します！🙅‍♂️🙅‍♂️🙅‍♂️")
				s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
			}
		}
	}
}

func getHTMLStr(url string) (string, error) {
	res, err := http.Get(url)
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
