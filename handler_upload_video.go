package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const (
	maxVidMemory = 1 << 30
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		fmt.Println(err)
	}

	if vid.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", err)
		return
	}

	err = r.ParseMultipartForm(maxVidMemory)
	if err != nil {
		fmt.Println(err)
	}

	videoFile, fileHeader, err := r.FormFile("video")
	if err != nil {
		fmt.Println(err)
	}

	defer videoFile.Close()

	mediaType := fileHeader.Header.Get("Content-Type")
	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		fmt.Println(err)
	}

	extension := ""
	switch mediaType {
	case "video/mp4":
		extension = "mp4"
	default:
		respondWithError(w, 400, "must upload an image", errors.New("no valid image found"))
		return
	}

	tmpVidFile, err := os.CreateTemp("", fmt.Sprintf("%v.%v", uuid.NewString(), extension))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}
	defer os.Remove(tmpVidFile.Name())
	defer tmpVidFile.Close()

	writeCount, err := io.Copy(tmpVidFile, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}
	fmt.Printf("vid bytes writen: %v\n", writeCount)

	tmpVidFile.Seek(0, io.SeekStart)

	aspectRatio, err := getVideoAspectRatio(tmpVidFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	filePrefix := getOrientaionFromRation(aspectRatio)

	fastStartVideoFilePath, err := processVideoForFastStart(tmpVidFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	vidFileToStore, err := os.Open(fastStartVideoFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	s3FileName := fmt.Sprintf("%v/%v.%v", filePrefix, uuid.NewString(), extension)
	fmt.Println(s3FileName)
	objInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &s3FileName,
		Body:        vidFileToStore,
		ContentType: &mediaType,
	}

	cfg.s3Client.PutObject(context.Background(), &objInput)

	//s3FileUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, s3FileName)
	s3FileInfo := EncodeS3VideoInfo(*objInput.Bucket, *objInput.Key)
	vid.VideoURL = &s3FileInfo

	cfg.db.UpdateVideo(vid)

	respondWithJSON(w, http.StatusNoContent, "")
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	buffer := &bytes.Buffer{}
	cmd.Stdout = buffer
	sb := &strings.Builder{}
	cmd.Stderr = sb

	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		fmt.Println(sb.String())
		return "other", err
	}

	output := cmdOuptut{}
	err = json.Unmarshal(buffer.Bytes(), &output)
	if err != nil {
		return "other", err
	}

	imgData := output.Streams[0]
	ratio := float64(imgData.Height) / float64(imgData.Width)
	fmt.Printf("Height: %v, Width: %v ,Ratio: %v\n", imgData.Height, imgData.Width, ratio)
	if ratio >= 1.7 && ratio <= 1.8 {
		return "9:16", nil //portrait
	} else if ratio >= 0.55 && ratio <= 0.57 {
		return "16:9", nil //landscape
	} else {
		return "other", nil
	}
}

func getOrientaionFromRation(ratio string) string {
	switch ratio {
	case "16:9":
		return "landscape"
	case "9:16":
		return "portrait"
	default:
		return "other"
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy",
		"-movflags", "faststart", "-f", "mp4", outputFilePath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputFilePath, nil
}

type cmdOuptut struct {
	Streams []ImageMetaData `json:"streams"`
}

type ImageMetaData struct {
	Width  int64 `json:"width"`
	Height int64 `json:"height"`
}
