package youtube

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type captionTrack struct {
	BaseURL    string `json:"baseUrl"`
	Name       struct {
		SimpleText string `json:"simpleText"`
	} `json:"name"`
}

type captionList struct {
	CaptionTracks []captionTrack `json:"captionTracks"`
}

type transcript struct {
	Text string `xml:",chardata"`
	Start string `xml:"start,attr"`
	Dur string `xml:"dur,attr"`
}

type videoMetadata struct {
	Title        string `json:"title"`
	ChannelName  string `json:"author_name"`
}

// GetCaptions retrieves captions from a YouTube video URL
func GetCaptions(url string) (string, error) {
	videoID := extractVideoID(url)
	if videoID == "" {
		return "", fmt.Errorf("invalid YouTube URL")
	}

	// Get the video page
	resp, err := http.Get("https://www.youtube.com/watch?v=" + videoID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch video page: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Extract captions data
	captionsData := extractCaptions(string(body))
	if captionsData == nil || len(captionsData.CaptionTracks) == 0 {
		return "", fmt.Errorf("no captions found for video")
	}

	// Get the first available caption track
	track := captionsData.CaptionTracks[0]
	
	// Fetch the actual captions
	return fetchSubtitles(track.BaseURL)
}

// GetCaptionsAndMetadata retrieves captions and metadata from a YouTube video URL
func GetCaptionsAndMetadata(url string) (content string, videoTitle string, channelTitle string, err error) {
	videoID := extractVideoID(url)
	if videoID == "" {
		return "", "", "", fmt.Errorf("invalid YouTube URL")
	}

	// Get the video page
	resp, err := http.Get("https://www.youtube.com/watch?v=" + videoID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch video page: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Extract metadata
	metadata := extractMetadata(string(body))
	if metadata == nil {
		return "", "", "", fmt.Errorf("failed to extract video metadata")
	}

	// Extract captions data
	captionsData := extractCaptions(string(body))
	if captionsData == nil || len(captionsData.CaptionTracks) == 0 {
		return "", "", "", fmt.Errorf("no captions found for video")
	}

	// Get the first available caption track
	track := captionsData.CaptionTracks[0]
	
	// Fetch the actual captions
	content, err = fetchSubtitles(track.BaseURL)
	if err != nil {
		return "", "", "", err
	}

	return content, metadata.Title, metadata.ChannelName, nil
}

func extractVideoID(url string) string {
	if strings.Contains(url, "youtube.com/watch?v=") {
		parts := strings.Split(url, "v=")
		if len(parts) < 2 {
			return ""
		}
		id := parts[1]
		if amp := strings.Index(id, "&"); amp != -1 {
			id = id[:amp]
		}
		return id
	} else if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) < 2 {
			return ""
		}
		id := parts[1]
		if slash := strings.Index(id, "/"); slash != -1 {
			id = id[:slash]
		}
		return id
	}
	return ""
}

func extractCaptions(html string) *captionList {
	parts := strings.Split(html, `"captions":`)
	if len(parts) < 2 {
		return nil
	}

	jsonPart := parts[1]
	end := strings.Index(jsonPart, `,"videoDetails`)
	if end == -1 {
		return nil
	}

	jsonPart = jsonPart[:end]
	jsonPart = strings.ReplaceAll(jsonPart, `\u0026`, "&")
	jsonPart = strings.ReplaceAll(jsonPart, `\`, "")

	var captionData struct {
		PlayerCaptionsTracklistRenderer captionList `json:"playerCaptionsTracklistRenderer"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &captionData); err != nil {
		return nil
	}

	return &captionData.PlayerCaptionsTracklistRenderer
}

func extractMetadata(html string) *videoMetadata {
	parts := strings.Split(html, `"videoDetails":`)
	if len(parts) < 2 {
		return nil
	}

	jsonPart := parts[1]
	end := strings.Index(jsonPart, `,"annotations`)
	if end == -1 {
		return nil
	}

	jsonPart = jsonPart[:end]
	jsonPart = strings.ReplaceAll(jsonPart, `\u0026`, "&")
	jsonPart = strings.ReplaceAll(jsonPart, `\`, "")

	var metadata videoMetadata
	if err := json.Unmarshal([]byte(jsonPart), &metadata); err != nil {
		return nil
	}

	return &metadata
}

func fetchSubtitles(baseURL string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch subtitles: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Transcript []transcript `xml:"text"`
	}

	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse subtitles XML: %v", err)
	}

	var transcriptText strings.Builder
	for _, t := range result.Transcript {
		transcriptText.WriteString(t.Text)
		transcriptText.WriteString(" ")
	}

	return transcriptText.String(), nil
}
