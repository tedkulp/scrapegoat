# Scrapegoat

A fast, efficient ROM scraper for EmulationStation that uses the ScreenScraper.fr API to gather game metadata and media files.

## Features

- **Accurate matching** - Uses CRC32, MD5, and SHA1 hashes to precisely identify ROM files
- **ZIP file support** - Automatically extracts and hashes ROM files from ZIP archives
- **Concurrent downloads** - Downloads multiple media files in parallel for faster processing
- **Smart caching** - Caches both game metadata and media files to minimize API usage
- **EmulationStation compatible** - Generates `gamelist.xml` in the correct format
- **Rate limit handling** - Built-in retry logic and exponential backoff for API reliability
- **Platform support** - Works with any platform supported by ScreenScraper.fr
- **Real-time quota tracking** - Shows actual API usage directly from ScreenScraper
- **Dry-run mode** - Test scraping without downloading files or making changes
- **Debug mode** - Detailed logging of all API calls for troubleshooting

## Prerequisites

1. **ScreenScraper.fr Account**
   - Register at https://www.screenscraper.fr/
   - Developer credentials are required for API access

2. **Go 1.21 or later**
   - Required for building from source

## Installation

### Build from source

```bash
git clone https://github.com/tedkulp/scrapegoat.git
cd scrapegoat
go build -o scrapegoat cmd/scraper/main.go
```

### Using Make

```bash
make build    # Build the binary
make install  # Install to /usr/local/bin
make help     # Show all available targets
```

### Install with go install

```bash
go install github.com/tedkulp/scrapegoat/cmd/scraper@latest
```

## Configuration

1. Copy the example configuration file:

```bash
cp config.yaml.example config.yaml
```

2. Edit `config.yaml` and add your ScreenScraper.fr credentials:

```yaml
screenscraper:
  dev_id: "YOUR_DEV_ID"
  dev_password: "YOUR_DEV_PASSWORD"
  user_id: "YOUR_USERNAME"
  user_password: "YOUR_PASSWORD"
  software_name: "scrapegoat"
```

3. The configuration includes platform definitions. System IDs can be found at:
   https://www.screenscraper.fr/systemelist.php

## Usage

Scrapegoat provides three main commands:

### 1. Scrape ROMs

Scrape ROM files and generate gamelist.xml:

```bash
scrapegoat scrape --platform <platform> --rom-dir <path>
```

**Flags:**
```
  -p, --platform string        Platform/system name (required)
  -r, --rom-dir string         ROM directory path (required)
  -o, --gamelist-dir string    Directory for gamelist.xml output (default: same as rom-dir)
  -m, --media-root-dir string  Root directory for media files (default: same as rom-dir)
      --cache-dir string       Cache directory for downloaded media (default: ~/.scrapegoat)
  -w, --workers int            Number of concurrent download workers (default: 4)
      --dry-run                Scan and query API but don't download or write files
  -v, --verbose                Verbose output with debug logging
      --config string          Config file (default: ./config.yaml or ~/.config/scrapegoat/config.yaml)
```

**Examples:**

```bash
# Scrape NES ROMs with default settings
scrapegoat scrape --platform nes --rom-dir ~/roms/nes

# Scrape with verbose output to see all API calls
scrapegoat scrape --platform snes --rom-dir ~/roms/snes --verbose

# Dry run to preview what would be scraped
scrapegoat scrape --platform gba --rom-dir ~/roms/gba --dry-run

# Use more workers for faster downloads
scrapegoat scrape --platform ps1 --rom-dir ~/roms/ps1 --workers 8

# Custom output directories
scrapegoat scrape --platform n64 --rom-dir ~/roms/n64 \
  --gamelist-dir ~/emulationstation/n64 \
  --media-root-dir ~/emulationstation/n64/media
```

### 2. List Available Platforms

Fetch and display all platforms from ScreenScraper:

```bash
scrapegoat list-platforms
```

This shows all available platforms with their IDs, slugs, names, and supported file extensions. Results are cached for 12 hours.

**Example output:**
```
ID    Slug                      Platform Name                  Extensions
----- ------------------------- ------------------------------ ----------
7     nes                       Nintendo NES                   .nes, .unf, .unif
4     snes                      Nintendo SNES                  .smc, .sfc, .swc
11    virtualboy                Nintendo virtualboy            .vb, .vboy, .bin
```

### 3. Check User Account Info

Display your ScreenScraper account information and API quota:

```bash
scrapegoat user-info
```

**Example output:**
```
=== Account Information ===
User ID: youruser
Level: 1
Contribution: 10
ROMs Associated: 0

=== API Quota ===
Requests Today: 1234
Max Requests Per Day: 100000
Max Threads: 2

Remaining Requests Today: 98766
```

## Output

Scrapegoat creates the following structure:

```
<rom-dir>/
├── gamelist.xml              # EmulationStation game metadata
├── Game1.zip                 # Your ROM files
├── Game2.zip
└── <media-type>/             # Media files organized by type
    ├── wheel/
    │   ├── Game1.png         # Logo/wheel art
    │   └── Game2.png
    ├── box2dfront/
    │   ├── Game1.png         # Box art
    │   └── Game2.png
    ├── screenshots/
    │   ├── Game1.png         # Screenshots
    │   └── Game2.png
    ├── titlescreens/
    │   ├── Game1.png         # Title screens
    │   └── Game2.png
    └── videos/
        ├── Game1.mp4         # Gameplay videos
        └── Game2.mp4
```

**Note:** EmulationStation finds media files automatically by filename convention. Media tags are not included in gamelist.xml as EmulationStation knows where to look for them.

## Caching

Scrapegoat uses smart caching to minimize API usage:

- **Game metadata**: Cached for 12 hours in `~/.scrapegoat/games/`
- **Media files**: Cached permanently in `~/.scrapegoat/cache/<platform>/`
- **Platform list**: Cached for 12 hours

Cache is organized by MD5 hash for efficient lookups. Cached items don't count against your API quota.

## Media Types

Scrapegoat downloads the following media types (when available):

- **wheel** - Logo/wheel art for game selection menus
- **box2dfront** - 2D box/cover art
- **screenshots** - In-game screenshots
- **titlescreens** - Title screen images
- **videos** - Gameplay video clips
- **3dboxes** - 3D box renders
- **backcovers** - Back cover art
- **manuals** - Game manuals (PDF)

Media files are organized in subdirectories matching EmulationStation's conventions.

## Platform Configuration

The `config.yaml` file includes many common platforms. To add a new platform:

1. Find the system ID at https://www.screenscraper.fr/systemelist.php
2. Add it to your config file:

```yaml
platforms:
  dreamcast:
    id: 23
    name: "Sega Dreamcast"
    extensions:
      - ".cdi"
      - ".gdi"
      - ".chd"
```

Or use `scrapegoat list-platforms` to see all available platforms with their correct IDs and extensions.

## Tips

- **API Rate Limits**: ScreenScraper enforces rate limits. The tool includes a 1-second delay between ROM queries and handles rate limiting automatically with retry logic.

- **Free vs Premium**: Free accounts have daily limits (typically 20,000-30,000 requests). Premium accounts have higher limits. Use `scrapegoat user-info` to check your quota.

- **Hash Accuracy**: The tool computes CRC32, MD5, and SHA1 hashes for the most accurate matching. This may take a few seconds per ROM but ensures correct identification.

- **ZIP Files**: When ROMs are compressed in ZIP archives, the tool automatically extracts and hashes the ROM file inside (not the ZIP container). This ensures accurate matching against ScreenScraper's database.

- **Cache Benefits**: On subsequent runs, cached ROMs skip API calls entirely. Media files are also cached, so you won't re-download files you already have.

- **Debug Mode**: Use `--verbose` to see detailed logs of all API calls, cache hits, redirects, and quota updates. Helpful for troubleshooting API usage.

- **API Call Accounting**: Each media file download through ScreenScraper's API endpoints may count as 2-3 API calls internally. The tool tracks actual usage from the API's response data.

## Troubleshooting

**"game not found in ScreenScraper database"**
- The ROM might not be in the database
- Try a different ROM dump or version
- Some homebrew/unofficial games may not be available
- Verify the platform ID is correct using `scrapegoat list-platforms`

**"rate limited by API"**
- Check your quota with `scrapegoat user-info`
- Wait for your quota to reset (usually resets daily)
- Reduce the number of workers with `--workers 2`
- Consider upgrading to a premium ScreenScraper account

**"failed to hash ROM"**
- Check file permissions
- Ensure the ROM file isn't corrupted
- Verify the file extension is supported for the platform
- For ZIP files, ensure they contain a valid ROM file

**High API usage**
- Use `--verbose` to see exactly what API calls are being made
- Check if you're re-scraping ROMs that are already cached
- Each media file download counts as ~2-3 API calls on ScreenScraper's end
- Consider scraping smaller batches if you're hitting limits

**Cache issues**
- Cache is stored in `~/.scrapegoat/`
- Game metadata expires after 12 hours
- Media files are cached permanently
- Delete cache directory to force re-download: `rm -rf ~/.scrapegoat/`

## License

MIT License - see LICENSE file for details

## Credits

- Uses the excellent [ScreenScraper.fr](https://www.screenscraper.fr/) API
- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
- Designed for [EmulationStation](https://github.com/RetroPie/EmulationStation)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
