package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	youtube "example.com/ytdl/youtube"
)

func intro() {
	txt := `
Go Get Youtube
==============
Simple Youtube video downloader

ytdownload -id=VIDEO_ID
`
	fmt.Println(txt)
	flag.Usage()
}

func printVideoMeta(video youtube.Video) {
	txt := `
	ID	: %s
	Title	: %s
	Author	: %s
	Views	: %d
	Rating	: %f`

	fmt.Printf(txt, video.Id, video.Title, video.Author, video.View_count, video.Avg_rating)
	fmt.Println("\nFormats:")

	for i := 0; i < len(video.Formats); i++ {
		fmt.Printf("\t%d\tItag %d\t%s\t%s\n",
			i, video.Formats[i].Itag, video.Formats[i].Quality, video.Formats[i].Video_type)
	}

	fmt.Println()
}

func getItag(max int) int {
	var i int
	for {
		fmt.Printf("Pick a format [0-%d]: ", max)
		_, err := fmt.Scanf("%d", &i)
		if err == nil && i >= 0 && i <= max {
			return i
		}
		fmt.Println("Invalid entry:", i)
	}
}

func downloadVideo(video youtube.Video, index int, option *youtube.Option, useYtDlp bool) error {
	ext := video.GetExtension(index)
	filename := fmt.Sprintf("%s.%s", video.Id, ext)

	var err error
	if useYtDlp {
		// Try yt-dlp first
		err = video.DownloadWithYtDlp(index, filename, option)
		if err != nil {
			fmt.Println("yt-dlp error:", err)
			fmt.Println("Falling back to direct download...")
			err = video.Download(index, filename, option)
		}
	} else {
		// Use direct download
		err = video.Download(index, filename, option)
	}
	
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Downloaded:", video.Filename)
	}
	return err
}

// parseVTT reads a VTT file and extracts the text content
func parseVTT(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(content), "\n")
	var textBuilder strings.Builder
	
	// Simple VTT parser: skip headers and timestamps
	// Timestamps look like: 00:00:00.000 --> 00:00:00.000
	timestampRe := regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3}\s-->\s\d{2}:\d{2}:\d{2}\.\d{3}`)
	
	// Regex to strip inline tags like <c>, <00:00:00.000>, </c>
	tagRe := regexp.MustCompile(`<[^>]*>`)
	
	seenLines := make(map[string]bool) // To deduplicate lines if needed

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "WEBVTT" || strings.HasPrefix(line, "NOTE") {
			continue
		}
		if timestampRe.MatchString(line) {
			continue
		}
		// Skip just numbers (often line IDs)
		if _, err := strconv.Atoi(line); err == nil {
			continue
		}
		
		// Strip inline tags
		cleanLine := tagRe.ReplaceAllString(line, "")
		cleanLine = strings.TrimSpace(cleanLine)
		
		if cleanLine == "" {
			continue
		}
		
		// Deduplicate consecutive identical lines (common in some subtitles)
		if !seenLines[cleanLine] {
			textBuilder.WriteString(cleanLine + " ")
			seenLines[cleanLine] = true
		}
	}
	
	return textBuilder.String(), nil
}

type MeetingResponse struct {
	Id int `json:"id"`
}

type SummaryResponse struct {
	Summary string `json:"summary"`
}

func createMeeting(title, rawText, apiUrl string) (int, error) {
	// Construct URL with query parameters
	// Note: This might hit URL length limits for long transcripts
	baseURL, _ := url.Parse(apiUrl)
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	baseURL.Path += "meetings/"
	
	params := url.Values{}
	params.Add("title", title)
	params.Add("raw_text", rawText)
	baseURL.RawQuery = params.Encode()

	req, err := http.NewRequest("POST", baseURL.String(), nil)
	if err != nil {
		return 0, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("create meeting failed: %s %s", resp.Status, string(bodyBytes))
	}

	var result MeetingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	
	return result.Id, nil
}

func summarizeMeeting(meetingId int, apiUrl string) (string, error) {
	baseURL, _ := url.Parse(apiUrl)
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	baseURL.Path += fmt.Sprintf("meetings/%d/summarize", meetingId)

	req, err := http.NewRequest("POST", baseURL.String(), nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("summarize failed: %s %s", resp.Status, string(bodyBytes))
	}

	var result SummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Summary, nil
}

func main() {
	video_id := flag.String("id", "", "YouTube video ID")
	resume := flag.Bool("resume", false, "Resume download")
	itag := flag.Int("itag", 0, "Select format by itag")
	rename := flag.Bool("rename", false, "Rename file using title")
	mp3 := flag.Bool("mp3", false, "Extract MP3 via ffmpeg")
	useYtDlp := flag.Bool("use-ytdlp", true, "Use yt-dlp for downloads (recommended)")
	transcript := flag.Bool("transcript", false, "Fetch transcript and get a AI Generated Summary")
	cookiesBrowser := flag.String("cookies-browser", "", "Use cookies from browser (e.g. 'chrome', 'firefox') to bypass 429 errors")
	apiUrl := flag.String("api-url", "https://granola-ai-app.onrender.com", "API Base URL")
	flag.Parse()

	if *video_id == "" && len(os.Args) < 2 {
		flag.Usage()
		return
	}

	if *video_id == "" {
		// If no ID provided via flag, check if it's the first argument
		if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
			*video_id = os.Args[1]
		}
	}

	fmt.Println("Fetching metadata...")
	video, err := youtube.Get(*video_id)
	if err != nil {
		fmt.Println("Error fetching metadata:", err)
		return
	}

	printVideoMeta(video)

	if *transcript {
		// Fetch transcript
		filename := video.Id // Use ID as base filename
		err := video.DownloadTranscript(filename, *cookiesBrowser)
		if err != nil {
			fmt.Println("Error fetching transcript:", err)
			os.Exit(1)
		} else {
			// Try to find the file to upload
			// It could be filename.en.vtt or filename.vtt
			uploadFile := filename + ".en.vtt"
			if _, err := os.Stat(uploadFile); os.IsNotExist(err) {
				uploadFile = filename + ".vtt"
			}
			
			if _, err := os.Stat(uploadFile); err == nil {
				fmt.Printf("Processing transcript from %s...\n", uploadFile)
				
				// Parse VTT to text
				text, err := parseVTT(uploadFile)
				if err != nil {
					fmt.Println("Error parsing VTT:", err)
				} else {
					fmt.Printf("Extracted %d characters of text.\n", len(text))
					
					// Create meeting
					fmt.Printf("Creating meeting on %s...\n", *apiUrl)
					meetingId, err := createMeeting(video.Title, text, *apiUrl)
					if err != nil {
						fmt.Println("Error creating meeting:", err)
					} else {
						fmt.Printf("Meeting created with ID: %d\n", meetingId)
						
						// Summarize meeting
						fmt.Println("Requesting summary...")
						summary, err := summarizeMeeting(meetingId, *apiUrl)
						if err != nil {
							fmt.Println("Error summarizing meeting:", err)
						} else {
							fmt.Println("\n=== SUMMARY ===")
							fmt.Println(summary)
							fmt.Println("===============")
							fmt.Printf("\nView this summary online at: https://granola-ai-app.vercel.app/meetings/%d\n", meetingId)
						}
					}
				}
			}
		}
		
		// If no specific download format was requested, exit
		if *itag == 0 {
			return
		}
	}

	var index int
	if *itag > 0 {
		idx, format := video.IndexByItag(*itag)
		if format == nil {
			fmt.Println("Unknown itag:", *itag)
			os.Exit(1)
		}
		index = idx
	} else {
		index = getItag(len(video.Formats) - 1)
	}

	option := &youtube.Option{
		Resume: *resume,
		Rename: *rename,
		Mp3:    *mp3,
	}

	err = downloadVideo(video, index, option, *useYtDlp)
	if err != nil {
		os.Exit(1)
	}
}
