# Go Get Youtube (go-ytdl)

A simple, fast, and reliable YouTube video downloader written in Go.

## Features

- 🚀 **Fast Metadata Fetching**: Uses a custom Go scraper to instantly fetch video details (title, author, views, formats).
- 📥 **Reliable Downloading**: Integrates with `yt-dlp` to handle YouTube's complex signature encryption and ensure successful downloads.
- 🛠 **Format Selection**: List available formats and choose which one to download.
- ⏯ **Resume Support**: Resume interrupted downloads.
- 🏷 **Auto-Renaming**: Option to automatically rename files based on video title.

## Prerequisites

- **Go**: 1.25 or higher
- **yt-dlp**: Required for downloading videos (due to YouTube's signature protection)

### Installing Prerequisites

**macOS (Homebrew):**
```bash
brew install yt-dlp
```

**Linux/Windows:**
Please refer to the [yt-dlp installation guide](https://github.com/yt-dlp/yt-dlp#installation).

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/go-ytdl.git
cd go-ytdl
```

2. Build the project:
```bash
go build -o ytdownload
```

## Usage

Run the tool with a video ID or URL:

```bash
./ytdownload -id=dQw4w9WgXcQ
```

### Command Line Options

| Flag | Description | Default |
|------|-------------|---------|
| `-id` | YouTube video ID or full URL | (Required) |
| `-itag` | Select format by itag number (skips interactive menu) | 0 |
| `-resume` | Resume interrupted download | false |
| `-rename` | Rename file using video title | false |
| `-use-ytdlp` | Use yt-dlp for downloads (recommended) | true |
| `-transcript` | Fetch transcript and summarize video | false |
| `-api-url` | API URL for summarization | https://granola-ai-app.onrender.com |
| `-cookies-browser` | Use browser cookies to bypass 429 errors (e.g. `chrome`, `firefox`) | "" |

### Examples

**Interactive Mode (Fetch metadata and pick format):**
```bash
./ytdownload -id=dQw4w9WgXcQ
```

**Summarize Video:**
```bash
./ytdownload -id=dQw4w9WgXcQ -transcript
```
The summary will be printed to the console and is also visible at [https://granola-ai-app.vercel.app/](https://granola-ai-app.vercel.app/).

**Summarize Video (with cookies to bypass 429):**
```bash
./ytdownload -id=dQw4w9WgXcQ -transcript -cookies-browser chrome
```

**Direct Download (Skip menu):**
```bash
./ytdownload -id=dQw4w9WgXcQ -itag=18
```

**Rename Output File:**
```bash
./ytdownload -id=dQw4w9WgXcQ -rename
```

## How It Works

1. **Metadata Phase**: The tool uses a custom Go-based scraper to fetch the YouTube video page and extract the `ytInitialPlayerResponse`. This allows it to quickly display video information without needing an API key.
2. **Download Phase**: When a download is requested, the tool invokes `yt-dlp` as a subprocess. This ensures that the download works even for videos with complex signature encryption that typically breaks simple downloaders.

### Summarization Workflow

When you use the `-transcript` flag, the tool performs the following automated steps:
1.  **Fetch Transcript**: Downloads subtitles/transcript from YouTube.
2.  **Hit the FAST API**: Uploads the transcript to the AI backend to create a new yt video entry.
3.  **Generate Summary**: Automatically triggers the AI summarization for that yt video.
4.  **Display**: Prints the summary and provides a link to view it online.

You do **not** need to provide a meeting ID; the tool handles the entire process automatically.

## License

MIT
