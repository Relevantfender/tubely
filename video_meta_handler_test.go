package main

import (
	"fmt"
	"testing"
)

func Test_handler_video_meta(t *testing.T) {

	t.Run("Get video aspect ratio", func(t *testing.T) {
		videoPath := "samples/boots-video-horizontal.mp4"
		value, err := getVideoAspectRatio(videoPath)
		if err != nil {
			t.Errorf("Expected no err, but got %v", err)
		}
		t.Logf("Ratio of video is: %s", value)
	})

	t.Run("Check if proper ratios", func(t *testing.T) {
		videoPath := "samples/boots-video-vertical.mp4"
		value, err := getVideoAspectRatio(videoPath)
		if err != nil {
			t.Errorf("Expected no err, but got %v", err)
		}
		t.Log(value)
		fmt.Println(value)
		if value != "9:16" {
			t.Errorf("Expected video to be vertical but is: %v", value)
		}

		videoPath = "samples/boots-video-horizontal.mp4"
		value, err = getVideoAspectRatio(videoPath)
		if err != nil {
			t.Errorf("Expected no err, but got %v", err)
		}
		t.Log(value)
		if value != "16:9" {
			t.Errorf("Expected video to be horizontal but is: %v", value)
		}

	})
}
