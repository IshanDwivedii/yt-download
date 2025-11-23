package main

import (
	"flag"
	"fmt"
	"os"

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

func downloadVideo(video youtube.Video, index int, option *youtube.Option) error {
	ext := video.GetExtension(index)
	filename := fmt.Sprintf("%s.%s", video.Id, ext)

	fmt.Printf("Downloading → %s\n", filename)

	err := video.Download(index, filename, option)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Downloaded:", video.Filename)
	}
	return err
}

func main() {
	video_id := flag.String("id", "", "YouTube video ID")
	resume := flag.Bool("resume", false, "Resume download")
	itag := flag.Int("itag", 0, "Select format by itag")
	rename := flag.Bool("rename", false, "Rename file using title")
	mp3 := flag.Bool("mp3", false, "Extract MP3 via ffmpeg")
	flag.Parse()

	if *video_id == "" && len(os.Args) < 2 {
		intro()
		return
	}

	fmt.Println("Fetching metadata...")

	video, err := youtube.Get(*video_id)
	if err != nil {
		video, err = youtube.Get(os.Args[1])
		if err != nil {
			fmt.Println("ERROR:", err)
			return
		}
	}

	printVideoMeta(video)

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

	err = downloadVideo(video, index, option)
	if err != nil {
		os.Exit(1)
	}
}
