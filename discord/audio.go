package discord

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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

var s3Persistence string = os.Getenv("S3_PERSISTENCE")

// OpusAudio is the barebones struct used to store an array of opus frames.
type OpusAudio struct {
	ByteArray [][]byte
}

// TODO: Commands maybe should be moved into their own file and solely audio utility functions live here
func playSound(command string, session *discordgo.Session, message *discordgo.MessageCreate) {
	// ToDo: Put command into struct
	tokens := strings.Split(command, " ")
	if len(tokens) < 2 {
		fmt.Println("Invalid command. Should be 2 tokens.")
		return
	}

	var opusData []byte
	if s3Persistence == "true" {
		opusData = getSoundS3(tokens[1])
	} else {
		opusData = getSoundLocal(tokens[1])
	}

	decodedFrames := gobDecodeOpusFrames(opusData)
	pipeOpusToDiscord(decodedFrames, session, message)
}

func listSounds() []string {
	if s3Persistence == "true" {
		return listSoundsS3()
	}
	return listSoundsLocal()
}

// Pull info from command - $rip <sound_name> <youtube_url> 0m0s 0m5s
func ripSound(ripCmd RipCommand) error {
	videoBuf, err := fetchVideoData(ripCmd.url)
	if err != nil {
		return err
	}
	opusFrames, err := convertToOpusFrames(videoBuf, ripCmd.start, ripCmd.duration)
	if err != nil {
		return err
	}
	encodedFrames := gobEncodeOpusFrames(opusFrames)

	var success bool
	if s3Persistence == "true" {
		success = putSoundS3(encodedFrames, ripCmd.name)
	} else {
		success = putSoundLocal(encodedFrames, ripCmd.name)
	}
	if !success {
		fmt.Println("Error saving audio file.")
	}
	return nil
}

func fetchVideoData(url string) (*bytes.Buffer, error) {
	vid, err := ytdl.GetVideoInfo(url)
	if err != nil {
		return nil, errors.New("Failed to get video info")
	}

	buf := new(bytes.Buffer)
	// ToDo: Customize formats?
	err = vid.Download(vid.Formats[0], buf)
	if err != nil {
		return nil, errors.New("Error downloading video")
	}

	return buf, nil
}

func convertToOpusFrames(videoBuf *bytes.Buffer, start string, duration string) ([][]byte, error) {
	run := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "-ss", start, "-t", duration, "pipe:1")
	ffmpegOut, _ := run.StdoutPipe()
	ffmpegIn, _ := run.StdinPipe()

	go func() {
		defer ffmpegIn.Close()
		ffmpegIn.Write(videoBuf.Bytes())
	}()

	// CDF: Did the buffer here make it chunk?
	ffmpegbuf := bufio.NewReader(ffmpegOut)

	err := run.Start()
	if err != nil {
		return nil, errors.New("Error converting video")
	}

	opusEncoder, _ := gopus.NewEncoder(frameRate, channels, gopus.Audio)
	opusFrames := make([][]byte, 0)
	for {
		// CDF: This represents a single frame. 20ms * 48 samples/ms * 2 channels
		frameBuf := make([]int16, frameSize*channels)

		err := binary.Read(ffmpegbuf, binary.LittleEndian, &frameBuf)
		if err == io.EOF {
			return opusFrames, nil
		} else if err != nil {
			return nil, errors.New("Error reading audio")
		}

		opusFrame, err := opusEncoder.Encode(frameBuf, frameSize, maxBytes)
		if err != nil {
			return nil, errors.New("Error encoding audio")
		}
		opusFrames = append(opusFrames, opusFrame)
	}
}

func putSoundLocal(buf bytes.Buffer, fileName string) bool {
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

func listSoundsLocal() []string {
	files, err := ioutil.ReadDir("./sounds")
	if err != nil {
		log.Fatal(err)
	}

	sounds := make([]string, 0)
	for _, f := range files {
		sounds = append(sounds, f.Name())
	}

	return sounds
}

func getSoundLocal(filename string) []byte {
	file, err := os.Open("sounds/" + filename)
	if err != nil {
		fmt.Println(filename, " does not exist.")
		return nil
	}
	// TODO: I can probably replace all this with a Reader or something.
	fileinfo, err := file.Stat()
	filesize := fileinfo.Size()
	buf := make([]byte, filesize)
	_, err = file.Read(buf)
	return buf
}

func gobEncodeOpusFrames(opusFrames [][]byte) (bytes.Buffer, error) {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)
	err := enc.Encode(OpusAudio{ByteArray: opusFrames})
	if err != nil {
		return nil, errors.New("Error gobbing frames")
	}
	return network, nil
}

func gobDecodeOpusFrames(data []byte) [][]byte {
	var (
		network    bytes.Buffer
		opusStruct OpusAudio
	)
	enc := gob.NewDecoder(&network)
	network.Write(data)

	err := enc.Decode(&opusStruct)
	if err != nil {
		log.Fatal("gobDecodeOpusFrames error:", err)
	}
	return opusStruct.ByteArray
}
