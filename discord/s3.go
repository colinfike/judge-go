package discord

import (
	"bytes"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

const (
	// BucketName is the main bucket that judge-go uses.
	BucketName string = "judge-go"
	// SoundClipRegex is the regex used to pull sound clip names from the bucet
	SoundClipRegex = "sound-clips/(.+)"
)

func listSoundsS3() []string {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	svc := s3.New(sess)

	resp, _ := svc.ListObjectsV2(&s3.ListObjectsV2Input{Bucket: aws.String(BucketName)})

	sounds := make([]string, 0)
	re := regexp.MustCompile(SoundClipRegex)
	for _, item := range resp.Contents {
		matches := re.FindSubmatch([]byte(*item.Key))
		if matches != nil {
			sounds = append(sounds, string(matches[1]))
		}
	}
	return sounds
}

func putSoundS3(sound bytes.Buffer, name string) bool {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	uploader := s3manager.NewUploader(sess)
	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(BucketName),
		Key:    aws.String("sound-clips/" + name),
		Body:   &sound,
	})
	if err != nil {
		return false
	}
	return true
}

func getSoundS3(name string) []byte {
	sess, _ := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	downloader := s3manager.NewDownloader(sess)

	buf := aws.NewWriteAtBuffer([]byte{})
	_, _ = downloader.Download(buf,
		&s3.GetObjectInput{
			Bucket: aws.String(BucketName),
			Key:    aws.String("sound-clips/" + name),
		})
	return buf.Bytes()
}
