package judgego

import (
	"bytes"
	"encoding/json"
	"log"
	"sync"
)

const reactionHistoryFilename = "reactionHistory.json"

type inductionMap struct {
	sync.RWMutex
	m map[string]bool
}

var reactionHistory = loadReactionHistory()

func alreadyInducted(channelID, messageID string) bool {
	reactionHistory.RLock()
	defer reactionHistory.RUnlock()
	if _, ok := reactionHistory.m[channelID+messageID]; ok {
		return true
	}
	return false
}

func inductMessage(channelID, messageID string) {
	reactionHistory.Lock()
	defer reactionHistory.Unlock()
	reactionHistory.m[channelID+messageID] = true
	saveReactionHistory()
}

func loadReactionHistory() inductionMap {
	reactionMap := make(map[string]bool)
	// TODO: Just assuming a failure here means the file doesn't exist in s3 for now. Should handle situation where it actually fails.
	b, err := getFromS3(reactionHistoryFilename)
	if err != nil {
		return inductionMap{m: reactionMap}
	}

	err = json.Unmarshal(b, &reactionMap)
	if err != nil {
		log.Fatal(err)
	}

	return inductionMap{m: reactionMap}
}

func saveReactionHistory() error {
	b, err := json.Marshal(reactionHistory.m)
	if err != nil {
		log.Fatal(err)
	}
	buf := bytes.NewBuffer(b)
	writeToS3(buf, reactionHistoryFilename)
	return nil
}
