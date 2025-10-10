# Scrapegoat

A fast, efficient ROM scraper for EmulationStation that uses the ScreenScraper.fr API to gather game metadata and media files.

## Features

- **Accurate matching** - Uses CRC32, MD5, and SHA1 hashes to precisely identify ROM files
- **ZIP file support** - Automatically extracts and hashes ROM files from ZIP archives
- **Concurrent downloads** - Downloads multiple media files in parallel for faster processing
- **EmulationStation compatible** - Generates `gamelist.xml` in the correct format
- **Rate limit handling** - Built-in retry logic and exponential backoff for API reliability
- **Platform support** - Works with any platform supported by ScreenScraper.fr
- **Dry-run mode** - Test scraping without downloading files or making changes

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
```

3. Add or modify platform definitions as needed. System IDs can be found at:
   https://www.screenscraper.fr/systemelist.php

## Usage

### Basic usage

Scrape ROMs for a specific platform:

```bash
./scrapegoat --platform nes --rom-dir /path/to/nes/roms
```

### Options

```
Flags:
  -p, --platform string      Platform/system name (required)
  -r, --rom-dir string       ROM directory path (required)
  -o, --output-dir string    Output directory for gamelist.xml and media (default: same as rom-dir)
      --temp-dir string      Temporary directory for downloads (default: /tmp/scrapegoat)
  -w, --workers int          Number of concurrent download workers (default: 4)
      --dry-run              Scan and query API but don't download or write files
  -v, --verbose              Verbose output
      --config string        Config file (default: ./config.yaml or ~/.config/scrapegoat/config.yaml)
  -h, --help                 Help for scrapegoat
```

### Examples

**Scrape SNES ROMs with verbose output:**
```bash
./scrapegoat --platform snes --rom-dir ~/RetroPie/roms/snes --verbose
```

**Dry run to see what would be scraped:**
```bash
./scrapegoat --platform n64 --rom-dir ~/roms/n64 --dry-run
```

**Scrape with custom output directory:**
```bash
./scrapegoat --platform gba --rom-dir ~/roms/gba --output-dir ~/emulationstation/gba
```

**Use more workers for faster downloads:**
```bash
./scrapegoat --platform ps1 --rom-dir ~/roms/ps1 --workers 8
```

## Output

Scrapegoat creates the following structure:

```
<output-dir>/
├── gamelist.xml          # EmulationStation game metadata
└── media/                # Downloaded media files
    ├── game1-box.png
    ├── game1-screenshot.png
    ├── game1-wheel.png
    ├── game2-box.png
    └── ...
```

## Platform Configuration

The `config.yaml` file includes common platforms. To add a new platform:

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

## Tips

- **API Rate Limits**: ScreenScraper has rate limits. The tool automatically handles rate limiting with retry logic, but adding a delay between ROMs helps avoid issues.

- **Free vs Premium**: Free accounts have lower rate limits. Consider upgrading to a premium account if you're scraping large collections.

- **Hash Accuracy**: The tool computes CRC32, MD5, and SHA1 hashes for the most accurate matching. This may take a few seconds per ROM but ensures correct identification.

- **ZIP Files**: When ROMs are compressed in ZIP archives, the tool automatically extracts and hashes the ROM file inside (not the ZIP container). This ensures accurate matching against ScreenScraper's database.

- **Media Types**: By default, the tool downloads:
  - Box art (2D box/cover)
  - Screenshots
  - Wheel art (logo)
  - Videos (when available)

## Troubleshooting

**"game not found in ScreenScraper database"**
- The ROM might not be in the database
- Try a different ROM dump or version
- Some homebrew/unofficial games may not be available

**"rate limited by API"**
- Wait a few minutes before retrying
- Reduce the number of workers
- Consider upgrading to a premium ScreenScraper account

**"failed to hash ROM"**
- Check file permissions
- Ensure the ROM file isn't corrupted
- Verify the file extension is supported

## License

MIT License - see LICENSE file for details

## Credits

- Uses the excellent [ScreenScraper.fr](https://www.screenscraper.fr/) API
- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
- Designed for [EmulationStation](https://github.com/RetroPie/EmulationStation)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
