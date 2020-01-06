package discord

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/rylio/ytdl"
	"layeh.com/gopus"
)

// TODO: I think the abstraction here could be modified a bit. I think the discord stuff probably belongs
// elsewhere and this file should handle audio manipulation.
// TODO: Optimize argument passing, make sure you are using references whereever possible

const (
	channels  int = 2                   // 1 for mono, 2 for stereo
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

type OpusAudio struct {
	ByteArray [][]byte
}

func playSound(command string, session *discordgo.Session, message *discordgo.MessageCreate) {
	// ToDo: Put command into struct
	tokens := strings.Split(command, " ")
	// if len(tokens) < 5 {
	// 	fmt.Println("Invalid command. Should be 5 tokens.")
	// 	return
	// }

	decodedFrames := gobDecodeOpusFrames(tokens[1])
	pipeOpusToDiscord(decodedFrames, session, message)
}

// Pull info from command - $rip <sound_name> <youtube_url> 0m0s 0m5s
func ripSound(command string) {

	// ToDo: Put command into struct
	tokens := strings.Split(command, " ")
	// if len(tokens) < 5 {
	// 	fmt.Println("Invalid command. Should be 5 tokens.")
	// 	return
	// }

	videoBuf := fetchVideoData(tokens[2])
	opusFrames := convertToOpusFrames(videoBuf)
	encodedFrames := gobEncodeOpusFrames(opusFrames)
	success := writeToFile(encodedFrames, tokens[1])
	if !success {
		fmt.Println("Error saving audio file.")
	}
}

// ToDO: Fix duplication?
func fetchVideoData(url string) *bytes.Buffer {
	vid, err := ytdl.GetVideoInfo(url)
	if err != nil {
		// ToDo: Have message be returned to users explaining this failed
		fmt.Println("Failed to get video info: ", url)
		return nil
	}
	buf := new(bytes.Buffer)
	// ToDo: Customize formats?
	err = vid.Download(vid.Formats[0], buf)
	if err != nil {
		fmt.Println("Error downloading video: ", err)
		return nil
	}

	return buf
}

func convertToOpusFrames(videoBuf *bytes.Buffer) [][]byte {
	run := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "pipe:1")
	ffmpegOut, _ := run.StdoutPipe()
	ffmpegIn, _ := run.StdinPipe()

	go func() {
		defer ffmpegIn.Close()
		ffmpegIn.Write(videoBuf.Bytes())
	}()

	// CDF: Did the buffer here make it chunk?
	ffmpegbuf := bufio.NewReader(ffmpegOut)

	_ = run.Start()

	opusEncoder, _ := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	opusFrames := make([][]byte, 10)
	for {
		fmt.Println("Looped", ffmpegbuf.Size())
		// CDF: This represents a single frame. 20ms * 48 samples/ms * 2 channels
		frameBuf := make([]int16, frameSize*channels)
		fmt.Println("before")
		err := binary.Read(ffmpegbuf, binary.LittleEndian, &frameBuf)
		fmt.Println("after")
		if err != nil {
			fmt.Println("EOF reached.")
			return opusFrames
		}
		opusFrame, _ := opusEncoder.Encode(frameBuf, frameSize, maxBytes)
		opusFrames = append(opusFrames, opusFrame)
	}
}

func gobEncodeOpusFrames(opusFrames [][]byte) bytes.Buffer {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)
	err := enc.Encode(OpusAudio{ByteArray: opusFrames})
	if err != nil {
		log.Fatal("gobEncodeOpusFrames error:", err)
	}
	return network
}

func writeToFile(buf bytes.Buffer, fileName string) bool {
	file, err := os.Create("sounds/" + fileName)
	if err != nil {
		fmt.Println("Error creating file: ", err)
		return false
	}
	defer file.Close()

	bytesWritten, err := file.Write(buf.Bytes())
	fmt.Printf("Wrote %v bytes.\n", bytesWritten)
	if err != nil {
		fmt.Println("Error writing audio: ", err)
		return false
	}

	return true
}

func gobDecodeOpusFrames(filename string) [][]byte {
	var (
		network    bytes.Buffer
		opusStruct OpusAudio
	)
	enc := gob.NewDecoder(&network)

	file, err := os.Open("sounds/" + filename)
	if err != nil {
		fmt.Println(filename, " does not exist.")
	}

	// TODO: I think I can just remove network. Leaving until v1 is done.
	fileinfo, err := file.Stat()
	filesize := fileinfo.Size()
	buffer := make([]byte, filesize)
	_, err = file.Read(buffer)
	network.Write(buffer)

	err = enc.Decode(&opusStruct)
	if err != nil {
		log.Fatal("gobDecodeOpusFrames error:", err)
	}
	return opusStruct.ByteArray
}

func pipeOpusToDiscord(opusFrames [][]byte, s *discordgo.Session, m *discordgo.MessageCreate) {

	vs, err := findUserVoiceState(s, m.Author.ID)
	// Connect to voice channel.
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
		fmt.Println("ByteSlice length: ", len(byteArray))
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
