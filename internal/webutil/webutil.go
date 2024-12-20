package webutil

import (
    "fmt"
    "io"
    "net/http"
    "strings"
    
    "github.com/JohannesKaufmann/html-to-markdown"
    "github.com/meain/refer/internal/youtube"
)

// IsURL checks if the given string is a URL
func IsURL(path string) bool {
    return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// IsYouTubeURL checks if the given URL is a YouTube URL
func IsYouTubeURL(url string) bool {
    return strings.Contains(url, "youtube.com/watch") || strings.Contains(url, "youtu.be/")
}

// FetchURL fetches a webpage and converts it to markdown
func FetchURL(url string) (string, error) {
    // Check if it's a YouTube URL
    if IsYouTubeURL(url) {
        return youtube.GetCaptions(url)
    }

    // Fetch the webpage
    resp, err := http.Get(url)
    if err != nil {
        return "", fmt.Errorf("failed to fetch URL: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("bad status code: %d", resp.StatusCode)
    }

    // Read the body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response body: %v", err)
    }

    // Create a converter
    converter := md.NewConverter("", true, nil)

    // Convert HTML to Markdown
    markdown, err := converter.ConvertString(string(body))
    if err != nil {
        return "", fmt.Errorf("failed to convert HTML to markdown: %v", err)
    }

    // Clean up the markdown
    markdown = strings.TrimSpace(markdown)
    
    return markdown, nil
}