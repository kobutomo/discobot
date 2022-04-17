package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"github.com/joho/godotenv"
	"github.com/kobutomo/discobot/dbservice"
)

type Contents struct {
	title string
	tag   string
	desc  string
}

type CrawlerMutex struct {
	sync.Mutex
	isLocked bool
}

var (
	adminID       string
	mainChannelID string
	cm            CrawlerMutex
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
	dg.AddHandler(generateMessageCreate(dbService))
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	err = dg.Open()
	if err != nil {
		log.Fatalln("Cannot connect,", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	dg.Close()
}

func ready(dbService *dbservice.DbService) func(s *discordgo.Session, event *discordgo.Ready) {
	return func(s *discordgo.Session, event *discordgo.Ready) {
		log.Println("Starts to inspect. v" + version)
		v := dbService.FindVersion(version)
		if v == "" {
			s.ChannelMessageSend(mainChannelID, fmt.Sprintf("`v%s` がリリースされました", version))
			dbService.InsertNewVersion(version)
		}
		s.UpdateGameStatus(0, "MAKE JORUJIO GREAT AGAIN")
	}
}

func generateMessageCreate(dbService *dbservice.DbService) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		ngReg, _ := regexp.Compile("^!addng ")
		rmngReg, _ := regexp.Compile("^!rmng ")
		showReg, _ := regexp.Compile("^!showng")
		verReg, _ := regexp.Compile("^!version")
		urlReg, _ := regexp.Compile("https?://")

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

		if urlReg.MatchString(m.Content) {
			if cm.isLocked {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46E忙しいので後にしてください！\U0001F645\U0001F645\U0001F645")
				s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
				return
			}
			cm.Lock()
			cm.isLocked = true
			contents, err := getDocument(m.Content)
			cm.isLocked = false
			cm.Unlock()
			if err != nil {
				log.Println(err)
				return
			}
			contain, word := containsNGWords(dbService, contents)
			if contain {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s ピピーッ！\U0001F46ENGワード `%s` を検出しました！削除します！\U0001F645\U0001F645\U0001F645", m.Author.Mention(), word))
				s.ChannelMessageDelete(m.ChannelID, m.Message.ID)
			}
		}

		if verReg.MatchString(m.Content) {
			ver := dbService.GetCurrentVersion()
			if ver == "" {
				ver = "0.0.0"
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("現在のバージョンは `v%s` です", ver))
		}

		if ngReg.MatchString(m.Content) {
			if m.Author.ID != adminID {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46E権限がありません！\U0001F645\U0001F645\U0001F645")
				return
			}
			var str string
			add := ""
			fmt.Sscanf(m.Content, "!addng %s %s", &str, &add)
			if strings.Contains(str, ",") || add != "" {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46Eフォーマット違反です！\U0001F645\U0001F645\U0001F645")
				return
			}
			if alreadyAddedNG(dbService, str) {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46E既に追加されているNGワードです！\U0001F645\U0001F645\U0001F645")
				return
			}
			addNG(dbService, str)
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("`%s` をNGワードに追加しました", str))
		}

		if rmngReg.MatchString(m.Content) {
			if m.Author.ID != adminID {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46E権限がありません！\U0001F645\U0001F645\U0001F645")
				return
			}
			var str string
			add := ""
			fmt.Sscanf(m.Content, "!rmng %s %s", &str, &add)
			if strings.Contains(str, ",") || add != "" {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46Eフォーマット違反です！\U0001F645\U0001F645\U0001F645")
				return
			}
			if !alreadyAddedNG(dbService, str) {
				s.ChannelMessageSend(m.ChannelID, "ピピーッ！\U0001F46E存在しないNGワードです！\U0001F645\U0001F645\U0001F645")
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

func getDocument(url string) (*Contents, error) {
	var title, tag, desc string
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var bool bool
	log.Println("checking start url:", url)
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.TextContent("//title", &title).Do(ctx); err != nil {
				return err
			}
			var descnodes []*cdp.Node
			var kwnodes []*cdp.Node
			if err := chromedp.Nodes("//*[@id='description']", &descnodes, chromedp.AtLeast(0)).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.Nodes("/html/head/meta[@name='keywords']", &kwnodes, chromedp.AtLeast(0)).Do(ctx); err != nil {
				return err
			}
			if len(descnodes) == 0 || len(kwnodes) == 0 {
				return nil
			}
			if err := chromedp.TextContent("//*[@id='description']", &desc).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.AttributeValue("/html/head/meta[@name='keywords']", "content", &tag, &bool).Do(ctx); err != nil {
				return err
			}
			return nil
		}),
	)
	log.Println("checking end")
	if err != nil {
		return nil, err
	}
	var contents = Contents{
		title: title,
		tag:   tag,
		desc:  desc,
	}
	return &contents, nil
}

func containsNGWords(dbService *dbservice.DbService, contents *Contents) (bool, string) {
	res, err := dbService.SelectAllNgs()
	if err != nil {
		log.Fatalln(err)
	}
	s := contents.desc + contents.tag + contents.title
	for _, word := range res {
		if strings.Contains(s, word) {
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
