package youtube

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	URL_META = "https://www.youtube.com/watch?v="
)

const (
	KB float64 = 1 << (10 * (iota + 1))
	MB
	GB
)

var (
	Formats = []string{"3gp", "mp4", "flv", "webm", "avi"}
)

type Video struct {
	Id, Title, Author, Keywords, Thumbnail_url string
	Avg_rating                                 float32
	View_count, Length_seconds                 int
	Formats                                    []Format
	Filename                                   string
}

type Format struct {
	Itag                     int
	Video_type, Quality, Url string
}

type Option struct {
	Resume bool
	Rename bool
	Mp3    bool
}

type playerResponse struct {
	VideoDetails struct {
		VideoID       string   `json:"videoId"`
		Title         string   `json:"title"`
		LengthSeconds string   `json:"lengthSeconds"`
		Keywords      []string `json:"keywords"`
		ViewCount     string   `json:"viewCount"`
		Author        string   `json:"author"`
		Thumbnail     struct {
			Thumbnails []struct {
				URL string `json:"url"`
			} `json:"thumbnails"`
		} `json:"thumbnail"`
		AverageRating float64 `json:"averageRating"`
	} `json:"videoDetails"`
	StreamingData struct {
		Formats         []streamFormat `json:"formats"`
		AdaptiveFormats []streamFormat `json:"adaptiveFormats"`
	} `json:"streamingData"`
}

type streamFormat struct {
	Itag            int    `json:"itag"`
	URL             string `json:"url"`
	SignatureCipher string `json:"signatureCipher"`
	MimeType        string `json:"mimeType"`
	Quality         string `json:"quality"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
	Bitrate         int    `json:"bitrate"`
}

func extractId(input string) (string, error) {
	u, err := url.Parse(input)
	if err != nil {
		return "", err
	}

	q := u.Query()
	if v := q.Get("v"); v != "" {
		return v, nil
	}
	return "", fmt.Errorf("no video ID")
}

func Get(video_id string) (Video, error) {
	if strings.Contains(video_id, "youtube.com/watch?") {
		video_id, _ = extractId(video_id)
	}

	query, err := fetchMeta(video_id)
	if err != nil {
		return Video{}, err
	}

	meta, err := parseMeta(video_id, query)
	if err != nil {
		return Video{}, err
	}

	return *meta, nil
}

func (video *Video) Download(index int, filename string, option *Option) error {
	var out *os.File
	var err error
	var offset int64
	var length int64

	if option.Resume {
		out, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		offset, err = out.Seek(0, os.SEEK_END)
		if err != nil {
			return err
		}
	} else {
		out, err = os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
	}
	defer out.Close()

	url := video.Formats[index].Url
	video.Filename = filename

	// Create HTTP client with proper headers
	client := &http.Client{}
	
	// HEAD request to get content length
	headReq, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return err
	}
	headReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	headReq.Header.Set("Referer", "https://www.youtube.com/")
	
	resp, err := client.Do(headReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	
	if resp.StatusCode == 403 {
		return errors.New("video forbidden")
	}
	size := resp.Header.Get("Content-Length")
	if size == "" {
		return errors.New("missing content length")
	}
	length, _ = strconv.ParseInt(size, 10, 64)

	if length > 0 {
		go printProgress(out, offset, length)
	}

	start := time.Now()
	
	// GET request to download video
	getReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	getReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	getReq.Header.Set("Referer", "https://www.youtube.com/")
	
	resp2, err := client.Do(getReq)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()

	if _, err = io.Copy(out, resp2.Body); err != nil {
		return err
	}

	fmt.Printf("Download took %s\n", time.Since(start))
	return nil
}

func abbr(b int64) string {
	s := float64(b)
	switch {
	case s > GB:
		return fmt.Sprintf("%.1fGB", s/GB)
	case s > MB:
		return fmt.Sprintf("%.1fMB", s/MB)
	case s > KB:
		return fmt.Sprintf("%.1fKB", s/KB)
	}
	return fmt.Sprintf("%d", b)
}

func printProgress(out *os.File, offset, length int64) {
	var clear string
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	start := time.Now()
	tail := offset

	for now := range ticker.C {
		d := now.Sub(start)
		d -= d % time.Second

		cur, _ := out.Seek(0, os.SEEK_CUR)
		speed := cur - tail
		percent := int(100 * cur / length)

		fmt.Printf("%s%s\t%s/%s\t%d%%\t%s/s\n",
			clear, d, abbr(cur), abbr(length), percent, abbr(speed),
		)

		tail = cur
		if tail >= length {
			break
		}

		if clear == "" && runtime.GOOS == "darwin" {
			clear = "\033[A\033[2K\r"
		}
	}
}

func (v *Video) GetExtension(index int) string {
	for _, f := range Formats {
		if strings.Contains(v.Formats[index].Video_type, f) {
			return f
		}
	}
	return "avi"
}

func (v *Video) IndexByItag(itag int) (int, *Format) {
	for i := range v.Formats {
		if v.Formats[i].Itag == itag {
			return i, &v.Formats[i]
		}
	}
	return 0, nil
}

func fetchMeta(video_id string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", URL_META+video_id, nil)
	if err != nil {
		return "", err
	}
	
	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch video page: status %d", resp.StatusCode)
	}

	b, _ := ioutil.ReadAll(resp.Body)
	return string(b), nil
}

func parseMeta(video_id, htmlContent string) (*Video, error) {
	// Extract ytInitialPlayerResponse from HTML
	re := regexp.MustCompile(`var ytInitialPlayerResponse = (\{.+?\});`)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return nil, errors.New("could not find player response in page")
	}

	var pr playerResponse
	if err := json.Unmarshal([]byte(matches[1]), &pr); err != nil {
		return nil, fmt.Errorf("failed to parse player response: %v", err)
	}

	thumbnailURL := ""
	if len(pr.VideoDetails.Thumbnail.Thumbnails) > 0 {
		thumbnailURL = pr.VideoDetails.Thumbnail.Thumbnails[0].URL
	}

	video := &Video{
		Id:            video_id,
		Title:         pr.VideoDetails.Title,
		Author:        pr.VideoDetails.Author,
		Keywords:      fmt.Sprint(pr.VideoDetails.Keywords),
		Thumbnail_url: thumbnailURL,
	}

	v, _ := strconv.Atoi(pr.VideoDetails.ViewCount)
	video.View_count = v

	l, _ := strconv.Atoi(pr.VideoDetails.LengthSeconds)
	video.Length_seconds = l

	// Parse formats from streamingData
	allFormats := append(pr.StreamingData.Formats, pr.StreamingData.AdaptiveFormats...)
	
	for _, f := range allFormats {
		videoURL := f.URL
		
		// If no direct URL, try to decode from signatureCipher
		if videoURL == "" && f.SignatureCipher != "" {
			// Parse the signature cipher
			cipherParams, err := url.ParseQuery(f.SignatureCipher)
			if err == nil {
				videoURL = cipherParams.Get("url")
				// Note: Full signature decryption would require executing JavaScript
				// For now, we'll try the URL as-is
			}
		}
		
		if videoURL == "" {
			// Skip formats without URL
			continue
		}
		
		// Determine quality label
		quality := f.Quality
		if quality == "" {
			if f.Height > 0 {
				quality = fmt.Sprintf("%dp", f.Height)
			} else {
				quality = "unknown"
			}
		}
		
		video.Formats = append(video.Formats, Format{
			Itag:       f.Itag,
			Video_type: f.MimeType,
			Quality:    quality,
			Url:        videoURL,
		})
	}

	if len(video.Formats) == 0 {
		return nil, errors.New("no formats available (may require signature decryption)")
	}

	return video, nil
}
