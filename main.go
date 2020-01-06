package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"bufio"
	"encoding/binary"
	"io"
	"os/exec"
	"strconv"
	"sync"

	"github.com/bwmarrin/discordgo"
	"layeh.com/gopus"
)

func main() {
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

	PlayAudioFile(dgv, "./tyler.mp4", make(chan bool))
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

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	if strings.Contains(m.Content, "trevor") {
		s.ChannelMessageSend(m.ChannelID, "I wish Trev wasn't around anymore")
	}
	if strings.Contains(m.Content, "Amen") {
		s.ChannelMessageSend(m.ChannelID, "Amen")
	}
	if strings.Contains(m.Content, "leggo") {
		playAudio(s, m)
	}
}

// DISCORDGO COPY PASTA

// NOTE: This API is not final and these are likely to change.

// Technically the below settings can be adjusted however that poses
// a lot of other problems that are not handled well at this time.
// These below values seem to provide the best overall performance
const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

var (
	speakers    map[uint32]*gopus.Decoder
	opusEncoder *gopus.Encoder
	mu          sync.Mutex
)

// OnError gets called by dgvoice when an error is encountered.
// By default logs to STDERR
var OnError = func(str string, err error) {
	prefix := "dgVoice: " + str

	if err != nil {
		os.Stderr.WriteString(prefix + ": " + err.Error())
	} else {
		os.Stderr.WriteString(prefix)
	}
}

// SendPCM will receive on the provied channel encode
// received PCM data into Opus then send that to Discordgo
func SendPCM(v *discordgo.VoiceConnection, pcm <-chan []int16) {
	if pcm == nil {
		return
	}

	var err error

	opusEncoder, err = gopus.NewEncoder(frameRate, channels, gopus.Audio)

	if err != nil {
		OnError("NewEncoder Error", err)
		return
	}

	for {

		// read pcm from chan, exit if channel is closed.
		recv, ok := <-pcm
		fmt.Println(len(recv))
		if ok {
			fmt.Println(recv[1918])
			fmt.Println(recv[1919])
		}

		fmt.Println(ok)
		fmt.Println("")
		if !ok {
			OnError("PCM Channel closed", nil)
			return
		}

		// try encoding pcm frame with Opus
		opus, err := opusEncoder.Encode(recv, frameSize, maxBytes)
		if err != nil {
			OnError("Encoding Error", err)
			return
		}

		// if v.Ready == false || v.OpusSend == nil {
		// 	// OnError(fmt.Sprintf("Discordgo not ready for opus packets. %+v : %+v", v.Ready, v.OpusSend), nil)
		// 	// Sending errors here might not be suited
		// 	fmt.Println("Here?")
		// 	return
		// }
		// send encoded opus data to the sendOpus channel
		v.OpusSend <- opus
	}
	fmt.Println("End of SendPCM")
}

// PlayAudioFile will play the given filename to the already connected
// Discord voice server/channel.  voice websocket and udp socket
// must already be setup before this will work.
func PlayAudioFile(v *discordgo.VoiceConnection, filename string, stop <-chan bool) {

	// Create a shell command "object" to run.

	run := exec.Command("ffmpeg", "-i", filename, "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	// run := exec.Command("ffmpeg", "-i", filename, "-acodec", "pcm_s16le", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	ffmpegout, err := run.StdoutPipe()
	if err != nil {
		OnError("StdoutPipe Error", err)
		return
	}

	// testPcm, _ := os.Open("pcmFileTest")
	// ffmpegbuf := bufio.NewReader(testPcm)
	ffmpegbuf := bufio.NewReaderSize(ffmpegout, 16384)

	// Starts the ffmpeg command
	err = run.Start()
	if err != nil {
		OnError("RunStart Error", err)
		return
	}

	go func() {
		<-stop
		err = run.Process.Kill()
	}()

	// Send "speaking" packet over the voice websocket
	err = v.Speaking(true)
	if err != nil {
		OnError("Couldn't set speaking", err)
	}

	// Send not "speaking" packet over the websocket when we finish
	defer func() {
		err := v.Speaking(false)
		if err != nil {
			OnError("Couldn't stop speaking", err)
		}
	}()

	// send is a channel of int16 arrays/slices
	send := make(chan []int16)
	defer close(send)

	go func() {
		fmt.Println("began async sendpcm call")
		SendPCM(v, send)
		fmt.Println("ended async sendpcm call")
	}()

	// const (
	// 	channels  int = 2                   // 1 for mono, 2 for stereo
	// 	frameRate int = 48000               // audio sampling rate
	// 	frameSize int = 960                 // uint16 size of each audio frame
	// 	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
	// )

	for {
		// read data from ffmpeg stdout
		audiobuf := make([]int16, frameSize*channels)
		err = binary.Read(ffmpegbuf, binary.LittleEndian, &audiobuf)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			send <- audiobuf
			return
		}
		if err != nil {
			OnError("error reading from ffmpeg stdout", err)
			return
		}

		// Send received PCM to the sendPCM channel
		select {
		case send <- audiobuf:
		}
	}
}
