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
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
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

// checkYtDlpInstalled checks if yt-dlp is available in PATH
func checkYtDlpInstalled() error {
	_, err := exec.LookPath("yt-dlp")
	if err != nil {
		return errors.New("yt-dlp not found. Install it with: brew install yt-dlp")
	}
	return nil
}

// DownloadWithYtDlp downloads a video using yt-dlp
func (video *Video) DownloadWithYtDlp(index int, filename string, option *Option) error {
	// Check if yt-dlp is installed
	if err := checkYtDlpInstalled(); err != nil {
		return err
	}

	// Build the video URL
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id)
	
	// Get the itag for the format
	itag := video.Formats[index].Itag
	
	// Build yt-dlp command
	args := []string{
		"-f", fmt.Sprintf("%d", itag), // Select format by itag
		"-o", filename,                 // Output filename
		videoURL,
	}
	
	// Add progress flag
	args = append(args, "--progress")
	
	fmt.Printf("Downloading → %s\n", filename)
	fmt.Println("Using yt-dlp for download...")
	
	// Create command
	cmd := exec.Command("yt-dlp", args...)
	
	// Connect stdout and stderr to show progress
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Run the command
	start := time.Now()
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("yt-dlp failed: %v", err)
	}
	
	video.Filename = filename
	fmt.Printf("\nDownload took %s\n", time.Since(start))
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

// extractPlayerURL extracts the player JavaScript URL from the HTML page
func extractPlayerURL(htmlContent string) (string, error) {
	// Look for the player URL in the HTML
	re := regexp.MustCompile(`"jsUrl":"(/s/player/[^"]+)"`)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		return "", errors.New("could not find player URL in page")
	}
	
	// Unescape the URL
	playerURL := strings.ReplaceAll(matches[1], `\/`, `/`)
	return "https://www.youtube.com" + playerURL, nil
}

// fetchPlayerCode downloads the player JavaScript code
func fetchPlayerCode(playerURL string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", playerURL, nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed to fetch player code: status %d", resp.StatusCode)
	}
	
	b, _ := ioutil.ReadAll(resp.Body)
	return string(b), nil
}

// extractDecryptFunction extracts the signature decryption function from player code
func extractDecryptFunction(playerCode string) (string, string, error) {
	// Try multiple patterns to find the decryption function
	patterns := []string{
		// Pattern 1: Standard pattern
		`\b([a-zA-Z0-9$]{2,})\s*=\s*function\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\)`,
		// Pattern 2: Alternative with semicolon
		`\b([a-zA-Z0-9$]{2,})\s*=\s*function\(\s*a\s*\)\s*\{\s*a\s*=\s*a\.split\(\s*""\s*\);`,
		// Pattern 3: With helper object reference
		`([a-zA-Z0-9$]+)=function\(a\)\{a=a\.split\(""\);([a-zA-Z0-9$]+)\.`,
		// Pattern 4: Newer YouTube pattern
		`\b([a-zA-Z0-9$]+)\s*=\s*function\([a-zA-Z]\)\{[a-zA-Z]=([a-zA-Z])\.split\(""\)`,
	}
	
	var funcName, helperName string
	var matches []string
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches = re.FindStringSubmatch(playerCode)
		if len(matches) >= 2 {
			funcName = matches[1]
			if len(matches) >= 3 {
				helperName = matches[2]
			}
			break
		}
	}
	
	if funcName == "" {
		return "", "", errors.New("could not find decryption function")
	}
	
	// If helper name not found, try to extract it from the function body
	if helperName == "" {
		helperPattern := fmt.Sprintf(`%s=function\([a-zA-Z]\)\{[a-zA-Z]=[a-zA-Z]\.split\(""\);([a-zA-Z0-9$]+)\.`, regexp.QuoteMeta(funcName))
		helperRe := regexp.MustCompile(helperPattern)
		helperMatches := helperRe.FindStringSubmatch(playerCode)
		if len(helperMatches) >= 2 {
			helperName = helperMatches[1]
		}
	}
	
	return funcName, helperName, nil
}

// extractHelperObject extracts the helper object code
func extractHelperObject(playerCode, helperName string) (string, error) {
	if helperName == "" {
		return "", nil
	}
	
	// Find the helper object definition with better pattern matching
	patterns := []string{
		fmt.Sprintf(`var %s=\{[^\}]+\}`, regexp.QuoteMeta(helperName)),
		fmt.Sprintf(`%s=\{[^\}]+\}`, regexp.QuoteMeta(helperName)),
		fmt.Sprintf(`var %s=\{[^;]+\};`, regexp.QuoteMeta(helperName)),
	}
	
	var match string
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match = re.FindString(playerCode)
		if match != "" {
			break
		}
	}
	
	if match == "" {
		// Try to find it manually by looking for the object start
		startPattern := fmt.Sprintf(`(var )?%s=\{`, regexp.QuoteMeta(helperName))
		startRe := regexp.MustCompile(startPattern)
		startIdx := startRe.FindStringIndex(playerCode)
		
		if startIdx != nil {
			// Find matching closing brace
			braceCount := 0
			inString := false
			escapeNext := false
			
			for i := startIdx[1] - 1; i < len(playerCode); i++ {
				c := playerCode[i]
				
				if escapeNext {
					escapeNext = false
					continue
				}
				
				if c == '\\' {
					escapeNext = true
					continue
				}
				
				if c == '"' || c == '\'' {
					inString = !inString
					continue
				}
				
				if !inString {
					if c == '{' {
						braceCount++
					} else if c == '}' {
						braceCount--
						if braceCount == 0 {
							match = playerCode[startIdx[0]:i+1]
							break
						}
					}
				}
			}
		}
	}
	
	if match == "" {
		return "", fmt.Errorf("could not find helper object: %s", helperName)
	}
	
	if !strings.HasSuffix(match, ";") {
		match += ";"
	}
	
	return match, nil
}

// extractFullDecryptFunction extracts the complete decryption function code
func extractFullDecryptFunction(playerCode, funcName string) (string, error) {
	// Find the function definition start
	startPattern := fmt.Sprintf(`%s=function\([a-zA-Z0-9]+\)\{`, regexp.QuoteMeta(funcName))
	startRe := regexp.MustCompile(startPattern)
	startIdx := startRe.FindStringIndex(playerCode)
	
	if startIdx == nil {
		return "", fmt.Errorf("could not find function definition: %s", funcName)
	}
	
	// Find matching closing brace
	braceCount := 0
	inString := false
	escapeNext := false
	var stringChar rune
	
	for i := startIdx[1] - 1; i < len(playerCode); i++ {
		c := rune(playerCode[i])
		
		if escapeNext {
			escapeNext = false
			continue
		}
		
		if c == '\\' {
			escapeNext = true
			continue
		}
		
		if c == '"' || c == '\'' {
			if !inString {
				inString = true
				stringChar = c
			} else if c == stringChar {
				inString = false
			}
			continue
		}
		
		if !inString {
			if c == '{' {
				braceCount++
			} else if c == '}' {
				braceCount--
				if braceCount == 0 {
					funcCode := playerCode[startIdx[0]:i+1]
					return "var " + funcCode + ";", nil
				}
			}
		}
	}
	
	return "", fmt.Errorf("could not find complete function definition: %s", funcName)
}

// decryptSignature decrypts a signature using the player code
func decryptSignature(signature, playerCode string) (string, error) {
	funcName, helperName, err := extractDecryptFunction(playerCode)
	if err != nil {
		return "", err
	}
	
	// Extract helper object if it exists
	helperCode, _ := extractHelperObject(playerCode, helperName)
	
	// Extract the main function
	funcCode, err := extractFullDecryptFunction(playerCode, funcName)
	if err != nil {
		return "", err
	}
	
	// Create JavaScript VM
	vm := goja.New()
	
	// Execute helper object and function
	if helperCode != "" {
		_, err = vm.RunString(helperCode)
		if err != nil {
			return "", fmt.Errorf("failed to execute helper code: %v", err)
		}
	}
	
	_, err = vm.RunString(funcCode)
	if err != nil {
		return "", fmt.Errorf("failed to execute function code: %v", err)
	}
	
	// Call the decryption function
	result, err := vm.RunString(fmt.Sprintf(`%s("%s")`, funcName, signature))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt signature: %v", err)
	}
	
	return result.String(), nil
}

// decipherURL deciphers a URL from signatureCipher
func decipherURL(signatureCipher, playerCode string) (string, error) {
	// Parse the signature cipher
	params, err := url.ParseQuery(signatureCipher)
	if err != nil {
		return "", err
	}
	
	baseURL := params.Get("url")
	signature := params.Get("s")
	
	if baseURL == "" {
		return "", errors.New("no URL in signature cipher")
	}
	
	if signature == "" {
		// No signature needed, return URL as-is
		return baseURL, nil
	}
	
	// Decrypt the signature
	decryptedSig, err := decryptSignature(signature, playerCode)
	if err != nil {
		return "", err
	}
	
	// Append the decrypted signature to the URL
	sigParam := params.Get("sp")
	if sigParam == "" {
		sigParam = "signature"
	}
	
	if strings.Contains(baseURL, "?") {
		return fmt.Sprintf("%s&%s=%s", baseURL, sigParam, url.QueryEscape(decryptedSig)), nil
	}
	return fmt.Sprintf("%s?%s=%s", baseURL, sigParam, url.QueryEscape(decryptedSig)), nil
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

	// Extract player URL and fetch player code for signature decryption
	var playerCode string
	playerURL, err := extractPlayerURL(htmlContent)
	if err == nil {
		playerCode, _ = fetchPlayerCode(playerURL)
	}

	// Parse formats from streamingData
	allFormats := append(pr.StreamingData.Formats, pr.StreamingData.AdaptiveFormats...)
	
	for _, f := range allFormats {
		videoURL := f.URL
		
		// If no direct URL, try to decipher from signatureCipher
		if videoURL == "" && f.SignatureCipher != "" {
			// Try to get base URL from signature cipher
			cipherParams, err := url.ParseQuery(f.SignatureCipher)
			if err == nil {
				videoURL = cipherParams.Get("url")
				
				// If we have player code, try to decrypt signature
				if playerCode != "" {
					signature := cipherParams.Get("s")
					if signature != "" {
						decipheredURL, err := decipherURL(f.SignatureCipher, playerCode)
						if err == nil {
							videoURL = decipheredURL
						}
						// If decryption fails, we still have the base URL (download will fail with 403)
					}
				}
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
		return nil, errors.New("no formats available")
	}

	return video, nil
}
