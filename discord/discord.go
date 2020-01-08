package discord

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	// DeleteDelay is the duration of time to wait before deleting a message
	DeleteDelay = 3 * time.Second
	// CensorRegex is a regex of all banned words
	CensorRegex = `\b(jon|wakeley|wakefest)\b`
	// HallOfFameChanID is the ChannelID of the Hall of Fame Channel
	HallOfFameChanID = "453637849234014219"
)

// Start is the main initialization function for the bot.
func Start() {
	token := os.Getenv("JUDGE_GO_BOT_TOKEN")
	dg, _ := discordgo.New("Bot " + token)

	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	err := dg.Open()
	if err != nil {
		fmt.Println(err)
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
		// Top level
		fmt.Println("Message does not exist.")
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
		fmt.Println(err)
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	var resp string
	switch cmd.(type) {
	case RipCommand:
		err = ripSound(cmd.(RipCommand))
		resp = "Sound successfully created!"
	default:
		fmt.Println("Default")
	}

	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	msg, err := s.ChannelMessageSend(m.ChannelID, resp)
	delayedDeleteMessage(s, msg)

	// fmt.Println(reflect.TypeOf(cmd) == RipCommand)
	// if (t, ok := cmd.(Ripcmd)) {
	// 	ripSound(t)
	// }
	// if strings.Contains(m.Content, "trevor") {
	// 	s.ChannelMessageSend(m.ChannelID, "I wish Trev wasn't around anymore")
	// }
	// if strings.Contains(m.Content, "$rip") {
	// 	ripSound(m.Content)
	// 	// TODO: Can create a function to handle response and deletion
	// 	message, _ := s.ChannelMessageSend(m.ChannelID, "Successfully created sound.")
	// 	delayedDeleteMessage(s, m.Message, message)
	// }
	// if strings.Contains(m.Content, "$play") {
	// 	playSound(m.Content, s, m)
	// 	delayedDeleteMessage(s, m.Message)
	// }
	// if strings.Contains(m.Content, "$list") {
	// 	sounds := listSounds()
	// 	joinedSounds := strings.Join(sounds, ", ")
	// 	s.ChannelMessageSend(m.ChannelID, "Available sounds: "+joinedSounds)
	// }

	// if containsBannedContent(m.Content) {
	// 	deleteMessage(s, m.Message)
	// 	message, _ := s.ChannelMessageSend(m.ChannelID, "That's banned content.")
	// 	delayedDeleteMessage(s, message)
	// }
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

func addToHallOfFame(s *discordgo.Session, m *discordgo.Message) {
	ts, _ := m.Timestamp.Parse()
	msgTxt := fmt.Sprintf("**Posted on %v by %v.**\n\n%v", ts.Format("January 2, 2006"), m.Author.Username, m.Content)
	_, _ = s.ChannelMessageSend(HallOfFameChanID, msgTxt)
}
