package discord

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
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

func playSound(command string, session *discordgo.Session, message *discordgo.MessageCreate) {
	// ToDo: Put command into struct
	tokens := strings.Split(command, " ")
	if len(tokens) < 2 {
		fmt.Println("Invalid command. Should be 2 tokens.")
		return
	}

	decodedFrames := gobDecodeOpusFrames(tokens[1])
	pipeOpusToDiscord(decodedFrames, session, message)
}

func listSounds() []string {
	if s3Persistence == "true" {
		return listSoundsS3()
	} else {
		return listSoundsLocal()
	}
}

// Pull info from command - $rip <sound_name> <youtube_url> 0m0s 0m5s
func ripSound(command string) {
	// ToDo: Put command into struct
	tokens := strings.Split(command, " ")
	if len(tokens) < 5 {
		fmt.Println("Invalid command. Should be 5 tokens.")
		return
	}

	videoBuf := fetchVideoData(tokens[2])
	start, duration := parseAudioLength(tokens[3], tokens[4])
	opusFrames := convertToOpusFrames(videoBuf, start, duration)
	encodedFrames := gobEncodeOpusFrames(opusFrames)

	var success bool
	if s3Persistence == "true" {
		success = putSoundS3(encodedFrames, tokens[1])
	} else {
		success = putSoundLocal(encodedFrames, tokens[1])
	}
	if !success {
		fmt.Println("Error saving audio file.")
	}
}

// TODO: A lot of this utility work should probably go into the tokenization functionality.
func parseAudioLength(start string, end string) (string, string) {
	startSec := convertTimeToSec(start)
	endSec := convertTimeToSec(end)
	return strconv.Itoa(startSec), strconv.Itoa(endSec - startSec)
}

// TODO: A lot of this utility work should probably go into the tokenization functionality.
func convertTimeToSec(timestamp string) int {
	re := regexp.MustCompile(`(\d*)m(\d*)s`)
	matches := re.FindSubmatch([]byte(timestamp))
	minutes, _ := strconv.Atoi(string(matches[1]))
	seconds, _ := strconv.Atoi(string(matches[2]))
	return minutes*60 + seconds
}

func fetchVideoData(url string) *bytes.Buffer {
	vid, err := ytdl.GetVideoInfo(url)
	if err != nil {
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

func convertToOpusFrames(videoBuf *bytes.Buffer, start string, duration string) [][]byte {
	run := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "-ss", start, "-t", duration, "pipe:1")
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
	opusFrames := make([][]byte, 0)
	for {
		// CDF: This represents a single frame. 20ms * 48 samples/ms * 2 channels
		frameBuf := make([]int16, frameSize*channels)
		err := binary.Read(ffmpegbuf, binary.LittleEndian, &frameBuf)
		if err != nil {
			return opusFrames
		}
		opusFrame, _ := opusEncoder.Encode(frameBuf, frameSize, maxBytes)
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

func gobEncodeOpusFrames(opusFrames [][]byte) bytes.Buffer {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)
	err := enc.Encode(OpusAudio{ByteArray: opusFrames})
	if err != nil {
		log.Fatal("gobEncodeOpusFrames error:", err)
	}
	return network
}

func gobDecodeOpusFrames(filename string) [][]byte {
	var (
		network    bytes.Buffer
		opusStruct OpusAudio
	)
	enc := gob.NewDecoder(&network)

	if s3Persistence == "true" {
		network.Write(fetchSoundsS3(filename).Bytes())
	} else {
		file, err := os.Open("sounds/" + filename)
		if err != nil {
			fmt.Println(filename, " does not exist.")
			return nil
		}

		// TODO: I think I can just remove network. Leaving until v1 is done.
		fileinfo, err := file.Stat()
		filesize := fileinfo.Size()
		buffer := make([]byte, filesize)
		_, err = file.Read(buffer)
		network.Write(buffer)
	}

	err := enc.Decode(&opusStruct)
	if err != nil {
		log.Fatal("gobDecodeOpusFrames error:", err)
	}
	return opusStruct.ByteArray
}
