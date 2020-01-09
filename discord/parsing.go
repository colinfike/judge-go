package discord

import (
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type RipCommand struct {
	name     string
	url      string
	start    string
	duration string
}
type PlayCommand struct {
	name string
}

const (
	ripPrefix         string = "$rip "
	ripCmdTokenCount  int    = 5
	timestampRegex    string = "^\\d+m\\d+s$"
	playPrefix        string = "$play "
	playCmdTokenCount int    = 5
)

// ParseMsg parses the message string and returns a struct based on the type of message it is.
func ParseMsg(msg string) (interface{}, error) {
	var (
		command interface{}
		err     error
	)
	if strings.HasPrefix(msg, ripPrefix) {
		command, err = parseRipCmd(msg)
	} else if strings.HasPrefix(msg, playPrefix) {
		command, err = parsePlayCmd(msg)
	}
	return command, err
}

func parseRipCmd(msg string) (RipCommand, error) {
	cmd := RipCommand{}

	tokens := strings.Split(msg, " ")
	if len(tokens) < 5 {
		return cmd, errors.New("Expected 5 tokens, received " + strconv.Itoa(len(tokens)))
	}
	cmd.name = tokens[1]

	if !isValidURL(tokens[2]) {
		return cmd, errors.New("Invalid URL passed")
	}
	cmd.url = tokens[2]

	if !isValidTimestamp(tokens[3]) || !isValidTimestamp(tokens[4]) {
		return cmd, errors.New("Invalid time stamps. Use XmYs form")
	}
	cmd.start, cmd.duration = parseAudioLength(tokens[3], tokens[4])

	return cmd, nil
}

func parsePlayCmd(msg string) (PlayCommand, error) {
	cmd := PlayCommand{}

	tokens := strings.Split(msg, " ")
	if len(tokens) < 2 {
		return cmd, errors.New("Expected 2 tokens, received " + strconv.Itoa(len(tokens)))
	}
	cmd.name = tokens[1]

	return cmd, nil
}

func isValidURL(testURL string) bool {
	_, err := url.ParseRequestURI(testURL)
	if err != nil {
		return false
	}
	return true
}

func isValidTimestamp(timestamp string) bool {
	re := regexp.MustCompile(timestampRegex)
	if re.FindIndex([]byte(timestamp)) != nil {
		return true
	}
	return false
}

func parseAudioLength(start string, end string) (string, string) {
	startSec := convertTimeToSec(start)
	endSec := convertTimeToSec(end)
	return strconv.Itoa(startSec), strconv.Itoa(endSec - startSec)
}

func convertTimeToSec(timestamp string) int {
	re := regexp.MustCompile(`(\d+)m(\d+)s`)
	matches := re.FindSubmatch([]byte(timestamp))
	minutes, _ := strconv.Atoi(string(matches[1]))
	seconds, _ := strconv.Atoi(string(matches[2]))
	return minutes*60 + seconds
}
