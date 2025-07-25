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
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
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

	const maxMemory = 10 << 20

	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")
	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error with the Content-Type header", err)
		return
	}
	fileExtension, err := getFileExtension(mediaType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No image extension found in the image", err)
		return
	}

	randBytes := make([]byte, 64)
	rand.Read(randBytes)

	videoName := base64.URLEncoding.EncodeToString(randBytes)
	filePath := filepath.Join(
		cfg.assetsRoot, fmt.Sprintf("%s.%s", videoName, fileExtension),
	)
	newFile, err := os.Create(filePath)

	_, err = io.Copy(newFile, file)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read the file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error while getting the video from db", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	video.ThumbnailURL = &filePath

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while updating the video metadata", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}

func getFileExtension(mediaType string) (string, error) {
	if mediaType != "" {
		data := strings.Split(strings.Trim(mediaType, " "), "/")
		if len(data) == 2 {
			return data[1], nil
		}
		return "", errors.New("No extension available in the header")
	}

	return "", errors.New("passed mediaType is an empty string")

}
