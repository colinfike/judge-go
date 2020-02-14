package judgego

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TODO: Create actual structs so we can use reciever functions. No reason to be passing session around as much I am.

// TODO: General error handling can be updated. I don't want to obfuscate the actual error for a user
// friendly message which is the current pattern. Can probably just extend Error and then print the
// actual error to log and respond to the user with the nice message.

const (
	// deleteDelay is the duration of time to wait before deleting a message
	deleteDelay = 8 * time.Second
	// censorRegex is a regex of all banned words
	censorRegex = `\b(wakeley|wakefest)\b`
	// inductionMinCount is the minimum amount of reactions a post must get to be inducted
	inductionMinCount = 3
	// reactorCount is the number of users to pull who reacted on a message
	reactorCount = 10
)

var (
	hallOfFameChanID  = os.Getenv("HALL_OF_FAME_ID")
	hallOfShameChanID = os.Getenv("HALL_OF_SHAME_ID")
	guildID           = os.Getenv("GUILD_ID")
	coolorRoles       = strings.Split(os.Getenv("ROLES"), ",")
)

// Start is the main initialization function for the bot.
func Start() {
	if os.Getenv("S3_PERSISTENCE") == "false" {
		initSoundDir()
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(messageCreate)
	dg.AddHandler(messageReactionAdd)

	err = dg.Open()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func messageReactionAdd(s *discordgo.Session, event *discordgo.MessageReactionAdd) {
	if alreadyInducted(event.ChannelID, event.MessageID) {
		return
	}

	message, err := s.ChannelMessage(event.ChannelID, event.MessageID)
	if err != nil {
		log.Printf("Message does not exist: %v", err)
	}
	// TODO: Can probably dry up with a map of emojis and functions to call if they meet the induction criteria
	for _, reaction := range message.Reactions {
		if reaction.Emoji.Name == "👌" {
			if reaction.Count >= inductionMinCount {
				reactors, err := getReactors(s, message, "👌")
				if err != nil {
					fmt.Println(err)
				}
				addToHallOfFame(s, message, reactors)
				inductMessage(message.ChannelID, message.ID)
			}
		} else if reaction.Emoji.Name == "💩" {
			if reaction.Count >= inductionMinCount {
				reactors, err := getReactors(s, message, "💩")
				if err != nil {
					fmt.Println(err)
				}
				addToHallOfShame(s, message, reactors)
				inductMessage(message.ChannelID, message.ID)
			}
		}
	}
}

func getReactors(s *discordgo.Session, message *discordgo.Message, emoji string) ([]string, error) {
	reactors := make([]string, 0)
	users, err := s.MessageReactions(message.ChannelID, message.ID, emoji, reactorCount)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		reactors = append(reactors, user.Username)
	}
	return reactors, nil
}

// commandResult contains the result of whatever resolving a command. It allows
// us to control the bot sending text or audio and/or deleting user messages.
type commandResult struct {
	resp          string
	audio         [][]byte
	deleteUserMsg bool
}

func resolveCommand(cmd interface{}) commandResult {
	var (
		cmdResult commandResult
		err       error
	)
	// ToDo: Make this default either via constructor or some Go funcctionality
	cmdResult.deleteUserMsg = true
	switch cmd.(type) {
	case ripCommand:
		err = ripSound(cmd.(ripCommand))
		cmdResult.resp = "Sound successfully created!"
	case playCommand:
		cmdResult.audio, err = playSound(cmd.(playCommand))
	case listCommand:
		cmdResult.resp, err = listSounds(cmd.(listCommand))
	case messageCommand:
		if containsBannedContent(cmd.(messageCommand)) {
			cmdResult.resp = "That's banned content."
		}
		cmdResult.deleteUserMsg = false
	default:
		log.Printf("Parsing broken: %+v", cmd)
	}

	if err != nil {
		cmdResult.resp = err.Error()
	}
	return cmdResult
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// TODO: Integrate into a command instead of hack.
	if strings.HasPrefix(m.Content, "mimic ") {
		member, err := getUserBySubstring(s, strings.TrimPrefix(m.Content, "mimic "))
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, "Member not found")
			return
		}
		deleteMessage(s, m.Message)
		s.ChannelMessageSend(m.ChannelID, member.Nick+": "+generateSentence(s, member.User.ID, m.ChannelID))
		return
	}

	// TODO: Integrate into a command instead of this even hackier code.
	if strings.HasPrefix(m.Content, "coolors https://coolors.co/") {
		re := regexp.MustCompile(`https://coolors.co/(.*)`)
		matches := re.FindSubmatch([]byte(m.Content))
		hexCodes := strings.Split(string(matches[1]), "-")

		guildRoles := make(map[string]*discordgo.Role)
		roles, _ := s.GuildRoles(guildID)
		for _, role := range roles {
			guildRoles[role.ID] = role
		}

		for index, hexCode := range hexCodes {
			role := guildRoles[coolorRoles[index]]
			decimalRGB, _ := strconv.ParseUint(hexCode, 16, 24)
			_, err := s.GuildRoleEdit(guildID, role.ID, role.Name, int(decimalRGB), role.Hoist, role.Permissions, role.Mentionable)
			if err != nil {
				fmt.Println(err)
			}
		}
		s.ChannelMessageDelete(m.ChannelID, m.ID)
	}

	cmd, err := parseMsg(m.Content)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, err.Error())
		return
	}

	cmdResult := resolveCommand(cmd)

	if len(cmdResult.audio) > 0 {
		err = pipeOpusToDiscord(cmdResult.audio, s, m)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, err.Error())
		}
	}
	if cmdResult.deleteUserMsg {
		deleteMessage(s, m.Message)
	}
	if len(cmdResult.resp) > 0 {
		msg, _ := s.ChannelMessageSend(m.ChannelID, cmdResult.resp)
		delayedDeleteMessage(s, msg)
	}
}

func containsBannedContent(messageCmd messageCommand) bool {
	re := regexp.MustCompile(censorRegex)
	if re.FindIndex([]byte(messageCmd.content)) != nil {
		return true
	}
	return false
}

func deleteMessage(s *discordgo.Session, m *discordgo.Message) {
	s.ChannelMessageDelete(m.ChannelID, m.ID)

}
func delayedDeleteMessage(s *discordgo.Session, messages ...*discordgo.Message) {
	time.Sleep(deleteDelay)
	for _, message := range messages {
		s.ChannelMessageDelete(message.ChannelID, message.ID)
	}
}

func pipeOpusToDiscord(opusFrames [][]byte, s *discordgo.Session, m *discordgo.MessageCreate) error {
	vs, err := findUserVoiceState(s, m.Author.ID)
	if err != nil {
		return errors.New("Couldn't find user voice channel")
	}
	dgv, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, true)
	if err != nil {
		return errors.New("Couldn't join voice channel")
	}
	defer dgv.Disconnect()

	err = dgv.Speaking(true)
	if err != nil {
		log.Println("Couldn't set speaking: ", err)
	}

	defer func() {
		err := dgv.Speaking(false)
		if err != nil {
			log.Println("Couldn't stop speaking: ", err)
		}
	}()

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < len(opusFrames); i++ {
		dgv.OpusSend <- opusFrames[rand.Intn(len(opusFrames))]
	}
	// for _, byteArray := range opusFrames {
	// 	dgv.OpusSend <- byteArray
	// }
	return nil
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

func addToHallOfFame(s *discordgo.Session, m *discordgo.Message, reactors []string) error {
	ts, err := m.Timestamp.Parse()
	if err != nil {
		log.Println("Discord messed up here: ", err.Error())
		return err
	}

	msgTxt := fmt.Sprintf("**Posted on %v by %v.**\n**Voted in by %v**\n\n%v", ts.Format("January 2, 2006"), m.Author.Username, strings.Join(reactors, ", "), m.Content)
	_, err = s.ChannelMessageSend(hallOfFameChanID, msgTxt)
	if err != nil {
		log.Println("Failed to create HoF message: ", err.Error())
		return err
	}

	return nil
}

func addToHallOfShame(s *discordgo.Session, m *discordgo.Message, reactors []string) error {
	ts, err := m.Timestamp.Parse()
	if err != nil {
		log.Println("Discord messed up here: ", err.Error())
		return err
	}

	msgTxt := fmt.Sprintf("**Posted in infamy on %v by %v.**\n**Voted in by %v**\n\n%v", ts.Format("January 2, 2006"), m.Author.Username, strings.Join(reactors, ", "), m.Content)
	_, err = s.ChannelMessageSend(hallOfShameChanID, msgTxt)
	if err != nil {
		log.Println("Failed to create HoF message: ", err.Error())
		return err
	}

	return nil
}

func initSoundDir() error {
	_, err := os.Stat("sounds/")
	if os.IsNotExist(err) {
		err = os.Mkdir("sounds", os.ModePerm)
	}
	if err != nil {
		return err
	}
	return nil
}

func getUserBySubstring(s *discordgo.Session, name string) (*discordgo.Member, error) {
	members, err := s.GuildMembers(guildID, "", 1000)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		if strings.Contains(strings.ToLower(member.Nick), strings.ToLower(name)) {
			return member, nil
		}
	}
	return nil, errors.New("User not found")
}
