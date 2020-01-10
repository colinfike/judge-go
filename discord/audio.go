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
	"sync"

	"github.com/rylio/ytdl"
	"layeh.com/gopus"
)

// OpusAudio is the barebones struct used to store an array of opus frames.
type OpusAudio struct {
	ByteArray [][]byte
}

const (
	channels  int = 2
	frameRate int = 48000               // audio sampling rate
	frameSize int = 960                 // uint16 size of each audio frame
	maxBytes  int = (frameSize * 2) * 2 // max size of opus data
)

var s3Persistence string = os.Getenv("S3_PERSISTENCE")
var cache = struct {
	sync.RWMutex
	m map[string][][]byte
}{m: make(map[string][][]byte)}

func checkCache(name string) ([][]byte, bool) {
	cache.RLock()
	val, ok := cache.m[name]
	cache.RUnlock()
	return val, ok
}

func putCache(name string, data [][]byte) {
	cache.Lock()
	cache.m[name] = data
	cache.Unlock()
}

// TODO: Commands maybe should be moved into their own file and solely audio utility functions live here
func playSound(playCmd PlayCommand) ([][]byte, error) {
	val, ok := checkCache(playCmd.name)
	if ok {
		return val, nil
	}

	var opusData []byte
	if s3Persistence == "true" {
		opusData = getSoundS3(playCmd.name)
	} else {
		opusData = getSoundLocal(playCmd.name)
	}

	decodedFrames := gobDecodeOpusFrames(opusData)
	putCache(playCmd.name, decodedFrames)
	return decodedFrames, nil
}

func listSounds(listCmd ListCommand) (string, error) {
	var (
		err    error
		sounds []string
	)
	if s3Persistence == "true" {
		sounds, err = listSoundsS3()
	} else {
		sounds, err = listSoundsLocal()
	}

	if err != nil {
		return "", err
	}

	return "Available Sounds: " + strings.Join(sounds, ", "), nil
}

// TODO: The code duplication here for error handling is unreal.
// Consider adding functionality to the functions so they return instantly
// if passed a nil value so we can a single error check at the end.
func ripSound(ripCmd RipCommand) error {
	videoBuf, err := fetchVideoData(ripCmd.url)
	if err != nil {
		return err
	}
	opusFrames, err := convertToOpusFrames(videoBuf, ripCmd.start, ripCmd.duration)
	if err != nil {
		return err
	}
	encodedFrames, err := gobEncodeOpusFrames(opusFrames)
	if err != nil {
		return err
	}

	if s3Persistence == "true" {
		err = putSoundS3(encodedFrames, ripCmd.name)
	} else {
		err = putSoundLocal(encodedFrames, ripCmd.name)
	}
	return err
}

func fetchVideoData(url string) (*bytes.Buffer, error) {
	vid, err := ytdl.GetVideoInfo(url)
	if err != nil {
		return nil, errors.New("Failed to get video info. Is the url valid?")
	}

	buf := new(bytes.Buffer)
	err = vid.Download(vid.Formats[0], buf)
	// err = vid.Download(vid.Formats.Best(ytdl.FormatResolutionKey)[0], buf)
	if err != nil {
		return nil, errors.New("Error downloading video")
	}
	return buf, nil
}

// TODO: Opportunity to tune this function a bit more I think.
func convertToOpusFrames(videoBuf *bytes.Buffer, start string, duration string) ([][]byte, error) {
	run := exec.Command("ffmpeg", "-i", "pipe:0", "-f", "s16le", "-ar", strconv.Itoa(frameRate), "-ac", strconv.Itoa(channels), "-ss", start, "-t", duration, "pipe:1")
	ffmpegOut, _ := run.StdoutPipe()
	ffmpegIn, _ := run.StdinPipe()

	go func() {
		defer ffmpegIn.Close()
		ffmpegIn.Write(videoBuf.Bytes())
	}()

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
			fmt.Println(err)
			return nil, errors.New("Error reading audio")
		}

		opusFrame, err := opusEncoder.Encode(frameBuf, frameSize, maxBytes)
		if err != nil {
			return nil, errors.New("Error encoding audio")
		}
		opusFrames = append(opusFrames, opusFrame)
	}
}

func putSoundLocal(buf *bytes.Buffer, fileName string) error {
	file, err := os.Create("sounds/" + fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(buf.Bytes())
	return err
}

func listSoundsLocal() ([]string, error) {
	files, err := ioutil.ReadDir("./sounds")
	if err != nil {
		return nil, errors.New("Can't read local sound directory")
	}

	sounds := make([]string, 0)
	for _, f := range files {
		sounds = append(sounds, f.Name())
	}

	return sounds, nil
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

func gobEncodeOpusFrames(opusFrames [][]byte) (*bytes.Buffer, error) {
	network := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(network)
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
