package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os/exec"

	"github.com/google/uuid"
	"github.com/relevantfender/tubely/internal/auth"
	"github.com/relevantfender/tubely/internal/database"
)

func (cfg *apiConfig) handlerVideoMetaCreate(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		database.CreateVideoParams
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

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't decode parameters", err)
		return
	}
	params.UserID = userID

	video, err := cfg.db.CreateVideo(params.CreateVideoParams)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create video", err)
		return
	}

	respondWithJSON(w, http.StatusCreated, video)
}

func (cfg *apiConfig) handlerVideoMetaDelete(w http.ResponseWriter, r *http.Request) {
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusForbidden, "You can't delete this video", err)
		return
	}

	err = cfg.db.DeleteVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't delete video", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerVideoGet(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid video ID", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

func (cfg *apiConfig) handlerVideosRetrieve(w http.ResponseWriter, r *http.Request) {
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

	videos, err := cfg.db.GetVideos(userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't retrieve videos", err)
		return
	}
	respondWithJSON(w, http.StatusOK, videos)
}

type Streams struct {
	Stream []StreamInfo `json:"streams"`
}

type StreamInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	args := []string{"-v", "error", "-print_format", "json", "-show_streams", filePath}
	cmd := exec.Command("ffprobe", args...)
	buffer := bytes.Buffer{}
	cmd.Stdout = &buffer
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Error while executing the ffprobe command: %w", err)
	}

	streamData := Streams{}
	err = json.Unmarshal(buffer.Bytes(), &streamData)

	if err != nil {
		return "", fmt.Errorf("Error while unmarshaling an ffprobe reply: %w", err)
	}

	width := streamData.Stream[0].Width
	height := streamData.Stream[0].Height

	actualRatio := float64(width) / float64(height)
	var targetRatio float64
	tolerance := 0.01

	if width != 0 && height != 0 {
		targetRatio = 16.0 / 9.0

		if math.Abs(actualRatio-targetRatio) < tolerance {
			return "16:9", nil

		}
		targetRatio = 9.0 / 16.0

		if math.Abs(actualRatio-targetRatio) < tolerance {
			return "9:16", nil
		}

		return "other", nil

	}
	return "", fmt.Errorf("error while processing aspect ratios")

}
func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	args := []string{"-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath}
	cmd := exec.Command("ffmpeg", args...)
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error while running command: %v", err)
	}
	return outputFilePath, nil
}
