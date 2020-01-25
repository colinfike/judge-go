package judgego

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
	"github.com/colinfike/mimic"
)

const minimumWords = 5

var generalChannelID = os.Getenv("MARKOV_CHANNEL_ID")

var markovCache = newSafeCache()

// TODO: Fix arguments here, passing too much along. Need structs and/or better message passing system.
func generateSentence(s *discordgo.Session, userID, channelID string) string {
	if markov, ok := markovCache.get(userID); ok {
		realMarkov := markov.(*mimic.MarkovChain)
		return realMarkov.Generate()
	}
	markov := mimic.NewMarkovChain(minimumWords)
	notification, _ := s.ChannelMessageSend(channelID, "Generating Markov chain...")
	markov.Train(*getUserMessages(s, userID))
	s.ChannelMessageDelete(channelID, notification.ID)
	markovCache.put(userID, markov)
	return markov.Generate()
}

func getUserMessages(s *discordgo.Session, ID string) *[]string {
	usrMsgs := make([]string, 0)
	lastMessageID := ""
	msgCount := 0
	for {
		msgs, err := s.ChannelMessages(generalChannelID, 100, lastMessageID, "", "")
		if len(msgs) == 0 || err != nil {
			break
		}
		for _, msg := range msgs {
			if msg.Author.ID != ID {
				continue
			}
			usrMsgs = append(usrMsgs, msg.Content)
		}
		lastMessageID = msgs[len(msgs)-1].ID
		msgCount++
		if msgCount%10 == 0 {
			fmt.Printf("%v messages pulled so far\n", msgCount*100)
		}
	}
	return &usrMsgs
}
