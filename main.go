package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/kobutomo/go-kobayashi-watcher/valapiservice"
)

func main() {
	/*local only code */
	err := godotenv.Load(fmt.Sprintf("./%s.env", os.Getenv("GO_ENV")))
	if err != nil {
		log.Fatal(err)
	}

	Token := os.Getenv("DISCORD_TOKEN")
	APIKey := os.Getenv("API_KEY")
	log.Println("Token: ", Token)
	if Token == "" {
		return
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		log.Fatalln("Cannot create Discord session,", err)
		return
	}

	vas := valapiservice.NewValAPIService(APIKey, "https://asia.api.riotgames.com/")

	dg.AddHandler(ready)

	dg.AddHandler(generateMessegaCreate(vas))

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
	s.UpdateStatus(0, "READY TO FIRE")
}

func generateMessegaCreate(vas *valapiservice.ValAPIService) func(s *discordgo.Session, m *discordgo.MessageCreate) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {

		if m.Author.ID == s.State.User.ID {
			return
		}

		// !Helloというチャットがきたら　「Hello」　と返します
		if string(m.Content[0]) == "!" {
			slice := strings.Split(m.Content, "#")

			if len(slice) == 2 {
				puuid, _ := vas.GetPuuid(slice[1], slice[0][1:])
				s.ChannelMessageSend(m.ChannelID, puuid)
			} else {
				s.ChannelMessageSend(m.ChannelID, "invalid format")
			}
		}
	}
}
