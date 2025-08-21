package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/relevantfender/tubely/internal/auth"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {

	const maxMemory = 1 << 30
	r.ParseMultipartForm(maxMemory)

	// get video ID
	pathValue := r.PathValue("videoID")
	videoID, err := uuid.Parse(pathValue)

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "No valid id in request", err)
		return
	}

	// get userID

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error while getting bearer token in handler upload video", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)

	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Error while validating token in handler upload video", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			respondWithError(w, http.StatusNotFound, "No entry for this videoID", err)
			return

		}
		respondWithError(w, http.StatusInternalServerError, "Error while getting the video from DB", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You do not have the permission to get this video", err)
		return
	}

	file, header, err := r.FormFile("video")
	defer file.Close()

	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error while getting the video from form", err)
		return
	}
	defer file.Close()

	mediatype, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))

	extension, err := getFileExtension(mediatype)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while getting the extension", err)
		return
	}
	if extension != "mp4" {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Wrong format, got %s, expected mp4", extension), err)
		return
	}

	temp, err := os.CreateTemp("", "tubely-upload.mp4")

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while creating a temp file in handler upload video", err)
	}
	defer temp.Close()
	defer os.Remove(temp.Name())

	if _, err := io.Copy(temp, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while saving the file", err)
	}

	temp.Seek(0, io.SeekStart)

	aspectRatio, err := getVideoAspectRatio(temp.Name())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while reading aspect ratio of the video", err)
		return
	}
	processedVideoPath, err := processVideoForFastStart(temp.Name())

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while processing video for fast startup", err)
		return
	}

	processedVideo, err := os.Open(processedVideoPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while opening processing video for fast startup", err)
		return
	}
	defer processedVideo.Close()

	processedVideo.Seek(0, io.SeekStart)
	defer os.Remove(processedVideo.Name())

	switch aspectRatio {
	case "16:9":
		aspectRatio = "landscape"
	case "9:16":
		aspectRatio = "portrait"
	}

	key := fmt.Sprintf("%s/%s.%s", aspectRatio, pathValue, extension)
	inputs := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        processedVideo,
		ContentType: &mediatype,
	}

	object, err := cfg.s3Client.PutObject(
		r.Context(),
		&inputs,
	)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while uploading a file to s3 bucket", err)
	}

	log.Printf("Uploaded to aws s3: %v", object)
	s3Url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)

	video.VideoURL = &s3Url
	if err := cfg.db.UpdateVideo(video); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error while saving s3 link to the db", err)
	}

	respondWithJSON(w, http.StatusCreated, s3Url)

}
