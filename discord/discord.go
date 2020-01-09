package discord

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TODO: General error handling can be updated. I don't want to obfuscate the actual error for a user
// friendly message which is the current pattern. Can probably just extend Error and then print the
// actual error to log and respond to the user with the nice message.

const (
	// DeleteDelay is the duration of time to wait before deleting a message
	DeleteDelay = 10 * time.Second
	// CensorRegex is a regex of all banned words
	CensorRegex = `\b(jon|wakeley|wakefest)\b`
	// HallOfFameChanID is the ChannelID of the Hall of Fame Channel
	HallOfFameChanID = "453637849234014219"
)

// Start is the main initialization function for the bot.
func Start() {
	token := os.Getenv("JUDGE_GO_BOT_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Println(err)
	}

	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	err = dg.Open()
	if err != nil {
		log.Println(err)
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageReactionAdd(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	message, err := s.ChannelMessage(event.ChannelID, event.MessageID)
	if err != nil {
		log.Printf("Message does not exist: %v", err)
	}
	for _, reaction := range message.Reactions {
		if reaction.Emoji.Name == "👌" {
			if reaction.Count > 2 {
				addToHallOfFame(s, message)
			}
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	cmd, err := ParseMsg(m.Content)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	var (
		resp string
		opus [][]byte
	)
	switch cmd.(type) {
	case RipCommand:
		err = ripSound(cmd.(RipCommand))
		resp = "Sound successfully created!"
	case PlayCommand:
		opus, err = playSound(cmd.(PlayCommand))
		if opus != nil {
			pipeOpusToDiscord(opus, s, m)
		}
	case ListCommand:
		resp, err = listSounds(cmd.(ListCommand))
	case MessageCommand:
		if containsBannedContent(cmd.(MessageCommand)) {
			resp = "That's banned content."
			deleteMessage(s, m.Message)
		}
	default:
		log.Println("Parsing broken by " + m.Content)
		return
	}

	// ToDo: Can consolidate this I think
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}
	if len(resp) > 0 {
		msg, _ := s.ChannelMessageSend(m.ChannelID, resp)
		delayedDeleteMessage(s, msg, m.Message)
	}
}

func containsBannedContent(messageCmd MessageCommand) bool {
	re := regexp.MustCompile(CensorRegex)
	if re.FindIndex([]byte(messageCmd.content)) != nil {
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

	dgv, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, true)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer dgv.Disconnect()

	err = dgv.Speaking(true)
	if err != nil {
		fmt.Println("Couldn't set speaking", err)
	}

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

func addToHallOfFame(s *discordgo.Session, m *discordgo.Message) {
	ts, _ := m.Timestamp.Parse()
	msgTxt := fmt.Sprintf("**Posted on %v by %v.**\n\n%v", ts.Format("January 2, 2006"), m.Author.Username, m.Content)
	_, _ = s.ChannelMessageSend(HallOfFameChanID, msgTxt)
}
