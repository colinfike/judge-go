package discord

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	// DeleteDelay is the duration of time to wait before deleting a message
	DeleteDelay = 3 * time.Second
	// CensorRegex is a regex of all banned words
	CensorRegex = `\b(jon|wakeley|wakefest)\b`
)

// Start is the main initialization function for the bot.
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
		ripSound(m.Content)
		// TODO: Can create a function to handle response and deletion
		message, _ := s.ChannelMessageSend(m.ChannelID, "Successfully created sound.")
		delayedDeleteMessage(s, m.Message, message)
	}
	if strings.Contains(m.Content, "$play") {
		playSound(m.Content, s, m)
		delayedDeleteMessage(s, m.Message)
	}
	if strings.Contains(m.Content, "$list") {
		sounds := listSounds()
		joinedSounds := strings.Join(sounds, ", ")
		s.ChannelMessageSend(m.ChannelID, "Available sounds: "+joinedSounds)
	}

	if containsBannedContent(m.Content) {
		deleteMessage(s, m.Message)
		message, _ := s.ChannelMessageSend(m.ChannelID, "That's banned content.")
		delayedDeleteMessage(s, message)
	}
}

func containsBannedContent(message string) bool {
	re := regexp.MustCompile(CensorRegex)
	if re.FindIndex([]byte(message)) != nil {
		return true
	}
	return false
}

func deleteMessage(s *discordgo.Session, m *discordgo.Message) {
	s.ChannelMessageDelete(m.ChannelID, m.ID)

}
func delayedDeleteMessage(s *discordgo.Session, messages ...*discordgo.Message) {
	time.Sleep(DeleteDelay)
	for _, message := range messages {
		s.ChannelMessageDelete(message.ChannelID, message.ID)
	}
}

func pipeOpusToDiscord(opusFrames [][]byte, s *discordgo.Session, m *discordgo.MessageCreate) {
	vs, err := findUserVoiceState(s, m.Author.ID)

	// NOTE: Setting mute to false, deaf to true.
	dgv, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, true)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer dgv.Disconnect()

	// Send "speaking" packet over the voice websocket
	err = dgv.Speaking(true)
	if err != nil {
		fmt.Println("Couldn't set speaking", err)
	}

	// Send not "speaking" packet over the websocket when we finish
	defer func() {
		err := dgv.Speaking(false)
		if err != nil {
			fmt.Println("Couldn't stop speaking", err)
		}
	}()

	for _, byteArray := range opusFrames {
		dgv.OpusSend <- byteArray
	}
}

func findUserVoiceState(session *discordgo.Session, userid string) (*discordgo.VoiceState, error) {
	for _, guild := range session.State.Guilds {
		for _, vs := range guild.VoiceStates {
			if vs.UserID == userid {
				return vs, nil
			}
		}
	}
	return nil, errors.New("Could not find user's voice state")
}
