package discord

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"

	"github.com/bwmarrin/discordgo"
	"github.com/rylio/ytdl"
	"layeh.com/gopus"
)

// ToDo
// 1. Actual centralized tokenization
// 2. Verify tokens are valid

const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

func playAudio(s *discordgo.Session, m *discordgo.MessageCreate) {

	vs, err := findUserVoiceState(s, m.Author.ID)
	// Connect to voice channel.
	// NOTE: Setting mute to false, deaf to true.
	dgv, err := s.ChannelVoiceJoin(m.GuildID, vs.ChannelID, false, true)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer dgv.Disconnect()

	ripAudio(dgv, "LOL")
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

func ripAudio(dgv *discordgo.VoiceConnection, command string) {
	// tokens := strings.Split(command, " ")
	// if len(tokens) < 5 {
	// 	// ToDo: Print error in channel
	// 	return
	// }

	vid, err := ytdl.GetVideoInfo("https://www.youtube.com/watch?v=JYpXEIffELg")
	if err != nil {
		fmt.Println("Failed to get video info")
		return
	}

	// file, _ := os.Create(vid.Title + ".mp4")
	// byteArray = make([]byte)
	buf := new(bytes.Buffer)
	fmt.Println(buf.Len())

	// defer file.Close()
	vid.Download(vid.Formats[0], buf)
	fmt.Println(buf.Len())

	convertFileToOpus(dgv, buf)

	// file, _ := os.Create("tylerbyler" + ".mp4")
	// defer file.Close()

	// file.Write(buf.Bytes())

	// Pull info from command - $rip <sound_name> <youtube_url> 0m0s 0m5s
	// Download audio from youtube
	// Convert to opus and save locally
}

func convertFileToOpus(dgv *discordgo.VoiceConnection, buf *bytes.Buffer) {
	run := exec.Command("ffmpeg", "-i", "tyler.mp4", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")

	// stdin, err := run.StdinPipe()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fmt.Println("convertFileToOpus 2")

	// go func() {
	// 	defer stdin.Close()
	// 	stdin.Write(buf.Bytes())
	// 	// stdin.Write(buf.Bytes())
	// 	// fmt.Println("convertFileToOpus 2.5")
	// 	// stdin.Close()
	// 	// fmt.Println("convertFileToOpus 2.75")
	// }()

	ffmpegout, _ := run.StdoutPipe()

	// CDF: Did the buffer here make it chunk?
	ffmpegbuf := bufio.NewReader(ffmpegout)
	fmt.Println("convertFileToOpus 3")

	// Starts the ffmpeg command
	_ = run.Start()

	// Send "speaking" packet over the voice websocket
	err := dgv.Speaking(true)
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

	// var opusEncoder *gopus.Encoder
	opusEncoder, _ := gopus.NewEncoder(frameRate, channels, gopus.Audio)

	// pcmBuffer := new(bytes.Buffer)

	pcmFile, _ := os.Create("tylerpcmtest")
	defer pcmFile.Close()

	for {
		fmt.Println("Looped")
		// read data from ffmpeg stdout
		audiobuf := make([]int16, frameSize*channels) //CDF: This represents a single frame. 20ms * 48 samples/ms * 2 channels
		err := binary.Read(ffmpegbuf, binary.LittleEndian, &audiobuf)
		opus, _ := opusEncoder.Encode(audiobuf, frameSize, maxBytes)
		pcmFile.Write(opus)
		dgv.OpusSend <- opus
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			fmt.Println("End reached")
			// fmt.Println(err)
			// pcmFile, err := os.Create("pcmFileTest")
			// if err != nil {
			// 	log.Fatal(err)
			// }
			// defer pcmFile.Close()
			// pcmFile.Write(pcmBuffer.Bytes())
			return
		}
		// pcmBuffer.Write(opus)
	}
}
