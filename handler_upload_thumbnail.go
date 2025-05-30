package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

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

	mediaType := fileHeader.Header.Get("Content-Type")
	thumbBytes, err := io.ReadAll(thumbnailFile)
	if err != nil {
		fmt.Println(err)
	}

	vid, err := cfg.db.GetVideo(videoID)
	if err != nil {
		fmt.Println(err)
	}

	if vid.UserID != userID {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	thumb64 := base64.StdEncoding.EncodeToString(thumbBytes)
	thumbDataUrl := fmt.Sprintf("data:%v;base64,%v", mediaType, thumb64)
	vid.ThumbnailURL = &thumbDataUrl

	cfg.db.UpdateVideo(vid)

	respondWithJSON(w, http.StatusOK, vid)
}
