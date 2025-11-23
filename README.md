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

### Examples

**Interactive Mode (Fetch metadata and pick format):**
```bash
./ytdownload -id=dQw4w9WgXcQ
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

## License

MIT
