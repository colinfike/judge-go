package judgego

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAudioLength(t *testing.T) {
	startTs := "0m10s"
	endTs := "1m10s"

	start, duration := parseAudioLength(startTs, endTs)

	assert.Equal(t, start, "10")
	assert.Equal(t, duration, "60")
}

var parseRipCmdFailTable = []string{
	"$rip testName https://www.youtube.com/watch?v=dFuUCpBbbHw 0m10s",
	"$rip testName www 0m10s 0m15s",
	"$rip testName https://www.youtube.com/watch?v=dFuUCpBbbHw xxx 1m10s",
	"$rip testName https://www.youtube.com/watch?v=dFuUCpBbbHw 0m10s 5mx",
}

func TestParseRipCmd(t *testing.T) {
	ripCmd := "$rip testName https://www.youtube.com/watch?v=dFuUCpBbbHw 0m1s 0m10s"

	parsedRipCmd, err := parseRipCmd(ripCmd)

	assert.Nil(t, err)
	assert.Equal(t, parsedRipCmd, ripCommand{"testName", "https://www.youtube.com/watch?v=dFuUCpBbbHw", "1", "9"})
}
func TestParseRipCmdMissingToken(t *testing.T) {
	for _, cmd := range parseRipCmdFailTable {
		_, err := parseRipCmd(cmd)
		assert.NotNil(t, err)
	}
}

func TestParsePlayCmd(t *testing.T) {
	playCmd := "$play dethklok"

	parsedPlayCmd, err := parsePlayCmd(playCmd)

	assert.Nil(t, err)
	assert.Equal(t, parsedPlayCmd, playCommand{"dethklok"})
}

var convertTimeToSecTestTable = []struct {
	in  string
	out int
}{
	{"0m0s", 0},
	{"0m5s", 5},
	{"1m0s", 60},
	{"01m00s", 60},
	{"20m5s", 1205},
}

func TestConvertTimeToSec(t *testing.T) {
	for _, testData := range convertTimeToSecTestTable {
		convertedTimestamp := convertTimeToSec(testData.in)
		assert.Equal(t, convertedTimestamp, testData.out)
	}
}
