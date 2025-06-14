package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignedClient := s3.NewPresignClient(s3Client)

	objInput := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	v4PresignedHttpReq, err := presignedClient.PresignGetObject(context.Background(),
		&objInput,
		s3.WithPresignExpires(expireTime))

	if err != nil {
		return "", err
	}

	return v4PresignedHttpReq.URL, nil
}

func (cfg *apiConfig) DbVideoToSignedVideo(video database.Video) (database.Video, error) {
	bucket, key, err := DecodeS3VideoInfo(*video.VideoURL)
	if err != nil {
		return video, fmt.Errorf("failed to get bucket and key values from video. %w", err)
	}

	url, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
	if err != nil {
		return video, fmt.Errorf("failed to generate PresignedUrl for video %v\n%w", video.ID, err)
	}

	video.VideoURL = &url

	return video, nil
}

func EncodeS3VideoInfo(bucket, key string) string {
	return bucket + "," + key
}

func DecodeS3VideoInfo(encodedS3VideoInfo string) (bucket, key string, err error) {
	parts := strings.Split(encodedS3VideoInfo, ",")
	videoOptionLength := len(parts)
	if videoOptionLength != 2 {
		return "", "", fmt.Errorf("expected 2 options but received %v", videoOptionLength)
	}

	return parts[0], parts[1], nil
}
