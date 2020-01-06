package discord

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

func Start() {
	token := os.Getenv("JUDGE_GO_BOT_TOKEN")
	dg, _ := discordgo.New("Bot " + token)

	dg.AddHandler(messageCreate)

	_ = dg.Open()

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	if strings.Contains(m.Content, "trevor") {
		s.ChannelMessageSend(m.ChannelID, "I wish Trev wasn't around anymore")
	}
	if strings.Contains(m.Content, "$rip") {
		playAudio(s, m)
	}
}
