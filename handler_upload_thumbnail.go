package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const (
	maxMemory = 10 << 20
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		fmt.Println(err)
	}

	thumbnailFile, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		fmt.Println(err)
	}
	defer thumbnailFile.Close()

	mediaType := fileHeader.Header.Get("Content-Type")
	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		fmt.Println(err)
	}

	extension := ""
	switch mediaType {
	case "image/png":
		extension = "png"
	case "image/jpeg":
		extension = "jpeg"
	default:
		respondWithError(w, 400, "must upload an image", errors.New("no valid image found"))
		return
	}

	rngBytes := make([]byte, 32)
	rand.Read(rngBytes)
	filename := fmt.Sprintf("%v.%v", base64.RawURLEncoding.EncodeToString(rngBytes), extension)
	filePath := filepath.Join(cfg.assetsRoot, filename)

	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println(err)
	}

	io.Copy(file, thumbnailFile)

	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		fmt.Println(err)
	}

	if vid.UserID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	tumbUrl := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, filename)
	vid.ThumbnailURL = &tumbUrl

	cfg.db.UpdateVideo(vid)

	respondWithJSON(w, http.StatusOK, vid)
}
