package main

import (
	"bytes"
	"fmt"
	"io"
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
	"github.com/kobutomo/discobot/dbservice"
)

var (
	adminID       string
	mainChannelID string
	version       string
)

func main() {
	// logging の設定
	logfile, _ := os.OpenFile("discobot.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	multiLogFile := io.MultiWriter(os.Stdout, logfile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetOutput(multiLogFile)

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

	dbService, err := dbservice.New("./ngwords.sql")
	if err != nil {
		log.Fatalln(err)
	}
	defer dbService.Close()
	err = dbService.Init()
	if err != nil {
		log.Fatalln(err.Error())
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalln("Cannot create Discord session,", err)
		return
	}

	dg.AddHandler(ready(dbService))
	dg.AddHandler(generateMessegaCreate(dbService))
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

func ready(dbService *dbservice.DbService) func(s *discordgo.Session, event *discordgo.Ready) {
	return func(s *discordgo.Session, event *discordgo.Ready) {
		log.Println("習近平 starts to inspect. v" + version)
		v := dbService.FindVersion(version)
		if v == "" {
			s.ChannelMessageSend(mainChannelID, fmt.Sprintf("習近平 `v%s` がリリースされました🇨🇳", version))
			dbService.InsertNewVersion(version)
		}
		s.UpdateStatus(0, "MAKE CHINA GREAT")
	}
}

func generateMessegaCreate(dbService *dbservice.DbService) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		ngReg, _ := regexp.Compile("^!addng ")
		rmngReg, _ := regexp.Compile("^!rmng ")
		showReg, _ := regexp.Compile("^!showng")
		verReg, _ := regexp.Compile("^!version")

		if showReg.MatchString(m.Content) {
			ngWords, err := dbService.SelectAllNgs()
			if err != nil {
				log.Fatalln(err)
			}
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
			contain, word := containsNGWords(dbService, html)
			if contain {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s ピピーッ！👮‍♂️NGワード `%s` を検出しました！削除します！🙅‍♂️🙅‍♂️🙅‍♂️", m.Author.Mention(), word))
				s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
			}
		}

		if verReg.MatchString(m.Content) {
			ver := dbService.GetCurrentVersion()
			if ver == "" {
				ver = "0.0.0"
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("現在のバージョンは `v%s` です🇨🇳", ver))
		}

		if ngReg.MatchString(m.Content) {
			if m.Author.ID != adminID {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️権限がありません！🙅‍♂️🙅‍♂️🙅‍♂️")
				return
			}
			var str string
			add := ""
			fmt.Sscanf(m.Content, "!addng %s %s", &str, &add)
			if strings.Contains(str, ",") || add != "" {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️フォーマット違反です！🙅‍♂️🙅‍♂️🙅‍♂️")
				return
			}
			if alreadyAddedNG(dbService, str) {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️既に追加されているNGワードです！🙅‍♂️🙅‍♂️🙅‍♂️")
				return
			}
			addNG(dbService, str)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` をNGワードに追加しました", str))
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
			if !alreadyAddedNG(dbService, str) {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！👮‍♂️存在しないNGワードです！🙅‍♂️🙅‍♂️🙅‍♂️")
				return
			}
			removeNG(dbService, str)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` をNGワードから削除しました", str))
		}
	}
}

func alreadyAddedNG(dbService *dbservice.DbService, str string) bool {
	res := dbService.FindByWord(str)
	return res != ""
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

func containsNGWords(dbService *dbservice.DbService, str string) (bool, string) {
	res, err := dbService.SelectAllNgs()
	if err != nil {
		log.Fatalln(err)
	}
	for _, word := range res {
		if strings.Contains(str, word) {
			return true, word
		}
	}
	return false, ""
}

func addNG(dbService *dbservice.DbService, word string) {
	_, err := dbService.InsertNg(word)
	if err != nil {
		log.Fatalln(err)
	}
}

func removeNG(dbService *dbservice.DbService, word string) {
	err := dbService.DeleteNg(word)
	if err != nil {
		log.Fatalln(err)
	}
}
