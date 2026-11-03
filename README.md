# Scrapegoat

A ROM scraper for EmulationStation. It identifies your ROMs by hash against the [ScreenScraper.fr](https://www.screenscraper.fr/) database, downloads box art, screenshots, logos and videos, and writes a `gamelist.xml`.

## Features

- **Accurate matching**: identifies ROMs by CRC32, MD5 and SHA1 hashes, with the filename as a fallback
- **ZIP support**: hashes the ROM inside a ZIP archive, not the archive itself
- **Concurrent downloads**: fetches media files in parallel with a configurable worker pool
- **Caching**: caches game lookups and media files, so re-runs use far less API quota
- **Any platform**: platform IDs and extensions are fetched from ScreenScraper automatically
- **Quota-aware**: shows your remaining daily API quota from ScreenScraper's own responses
- **Artwork generation**: optionally composes custom images (miximages) from downloaded media; see [ARTWORK.md](ARTWORK.md)
- **Dry-run mode**: queries the API without downloading or writing anything

## Quick start

1. Download the archive for your OS from [GitHub Releases](https://github.com/tedkulp/scrapegoat/releases) and extract `scrapegoat`.
2. Create a config file with your ScreenScraper credentials (see [Configuration](#configuration)):
   ```bash
   cp config.yaml.example config.yaml
   ```
3. Try a dry run:
   ```bash
   ./scrapegoat scrape --platform nes --rom-dir ~/roms/nes --dry-run
   ```
4. Scrape for real:
   ```bash
   ./scrapegoat scrape --platform nes --rom-dir ~/roms/nes
   ```

> [!WARNING]
> At the end of a run, `scrape` lists the ROMs it could not match (not found, marked non-game, or failed) and **asks whether to delete them**. Pass `--auto-delete=false` to always keep them without a prompt.

## Prerequisites

- A [ScreenScraper.fr](https://www.screenscraper.fr/) account (user credentials)
- ScreenScraper developer credentials (`dev_id` and `dev_password`)
- Go 1.23 or later, only if you build from source

## Installation

### Prebuilt binaries

Releases include archives for Linux, macOS and Windows (x86_64 and arm64): https://github.com/tedkulp/scrapegoat/releases

### Build from source

```bash
git clone https://github.com/tedkulp/scrapegoat.git
cd scrapegoat
make build    # same as: go build -o scrapegoat cmd/scraper/main.go
```

Run `make help` to see all targets (`test`, `lint`, `install`, ...).

### go install

```bash
go install github.com/tedkulp/scrapegoat/cmd/scraper@latest
```

> [!NOTE]
> `go install` (and `make install`) put the binary in `$GOPATH/bin` under the name **`scraper`**, taken from the package directory. Rename it to `scrapegoat` if you want the commands below to match.

## Configuration

Scrapegoat reads `./config.yaml` first, then `~/.config/scrapegoat/config.yaml`. Pass `--config <path>` to use another file.

```bash
cp config.yaml.example config.yaml
```

Fill in your credentials:

```yaml
screenscraper:
  dev_id: "YOUR_DEV_ID"
  dev_password: "YOUR_DEV_PASSWORD"
  user_id: "YOUR_USERNAME"
  user_password: "YOUR_PASSWORD"
  software_name: "scrapegoat/1.0"
```

`config.yaml.example` also documents the optional `output`, `platforms` and `artwork` sections.

## Usage

### Scrape ROMs

```bash
scrapegoat scrape --platform <platform> --rom-dir <path>
```

| Flag | Default | Description |
|---|---|---|
| `-p, --platform` | (required) | Platform slug, e.g. `nes`. See `list-platforms`. |
| `-r, --rom-dir` | (required) | ROM directory |
| `-o, --gamelist-dir` | rom-dir | Where `gamelist.xml` is written |
| `-m, --media-root-dir` | rom-dir | Root directory for media subdirectories |
| `--cache-dir` | `~/.scrapegoat` | Cache for the platform catalog, game lookups and media |
| `-w, --workers` | `4` | Concurrent media download workers |
| `--cache-hours` | `12` | Hours before cached platforms and game lookups expire |
| `--force-redownload` | off | Fetch every media file from ScreenScraper again, skipping both the media root and the media cache. Counts against the daily quota. |
| `--artwork` | from config | `true` or `false` to override `artwork.enabled` |
| `--auto-delete` | prompt | `true` deletes unmatched ROMs without asking, `false` keeps them |
| `--dry-run` | off | Query the API but download and write nothing |
| `-v, --verbose` | off | Log every API request (passwords masked), cache hit and download |
| `--config` | see above | Config file path |

`--rom-dir`, `--gamelist-dir`, `--media-root-dir` and `--cache-dir` accept a `{platform}` placeholder, which is replaced by the `--platform` value:

```bash
for p in nes snes gba; do
  scrapegoat scrape --platform "$p" --rom-dir "$HOME/roms/{platform}" --auto-delete=false
done
```

More examples:

```bash
# Verbose output to see every API call
scrapegoat scrape --platform snes --rom-dir ~/roms/snes --verbose

# More download workers
scrapegoat scrape --platform psx --rom-dir ~/roms/psx --workers 8

# Separate output directories
scrapegoat scrape --platform n64 --rom-dir ~/roms/n64 \
  --gamelist-dir ~/.emulationstation/gamelists/n64 \
  --media-root-dir ~/.emulationstation/downloaded_media/n64
```

### List platforms

```bash
scrapegoat list-platforms
```

Prints every ScreenScraper platform with its ID, slug, name and file extensions. Results are cached for 12 hours.

```
ID    Slug                      Platform Name                  Extensions
----- ------------------------- ------------------------------ ----------
7     nes                       Nintendo NES                   .nes, .unf, .unif
4     snes                      Nintendo SNES                  .smc, .sfc, .swc
11    virtualboy                Nintendo virtualboy            .vb, .vboy, .bin
```

### Check account and quota

```bash
scrapegoat user-info
```

```
=== Account Information ===
User ID: youruser
Level: 1
Contribution: 10
ROMs Associated: 0
Uploads: 0

=== API Quota ===
Requests Today: 1234
Max Requests Per Day: 20000
Max Requests Per Minute: 1024
Failed Requests Today: 12 / 2000
Max Threads: 1

Remaining Requests Today: 18766
```

### Show version

```bash
scrapegoat version
```

## Output

```
<media-root-dir>/            # defaults to rom-dir
├── 3dboxes/Game1.png
├── backcovers/Game1.png
├── box2dfront/Game1.png
├── manuals/Game1.pdf
├── physicalmedia/Game1.png
├── screenshots/Game1.png
├── titlescreens/Game1.png
├── videos/Game1.mp4
└── wheel/Game1.png
<gamelist-dir>/gamelist.xml  # defaults to rom-dir
```

Each media file is named after its ROM without the extension. A type is skipped when ScreenScraper has no media for it. Media regions are preferred in the order us, wor, eu, jp.

`gamelist.xml` holds names, descriptions, ratings, release dates, developers, publishers, genres and player counts. It has no media tags: EmulationStation finds media by directory and filename.

Media files that already exist in the media root are kept, and files in the media cache are copied from there without an API request. `--force-redownload` skips both and fetches every media file from ScreenScraper again, overwriting the cached and placed copies; those downloads count against your daily quota. `--dry-run` reports how many files would be downloaded, copied from the cache or kept, and applies `--force-redownload` the same way.

A media file that fails to download is reported and counted in the `Media:` line as failed; the ROM itself still counts as scraped.

With artwork enabled, each artwork output writes `<media-root-dir>/<output_dir>/<rom-name>.<format>`; see [ARTWORK.md](ARTWORK.md).

## Caching

All caches live under `--cache-dir` (default `~/.scrapegoat`):

| What | Where | Expires |
|---|---|---|
| Platform list | `platforms.json` | after `--cache-hours` (12) |
| Game lookups, including "not found" | `games/` | after `--cache-hours` (12) |
| Media files | `cache/<platform>/<media-type>/` | never |
| Composed artwork | `artwork-cache/` | never; rebuilt when an output's settings or the media its layers use change |

Cache hits make no API request and do not count against your quota. Each cached media file has a `.json` sidecar beside it recording which ScreenScraper media it is (system, game and media name, never credentials). When a lookup selects different media for a ROM, such as another region, the cached file is downloaded again. A cached file without a sidecar is trusted and given one. Delete the directory to start fresh: `rm -rf ~/.scrapegoat/`.

## Platforms

Platforms come from the ScreenScraper API, so you usually don't need to configure any. To override one or work offline, define it in `config.yaml` (IDs are listed at https://www.screenscraper.fr/systemelist.php):

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

Entries in `config.yaml` take priority over the cache and the API.

## API limits

- Scrapegoat waits at least 1 second between HTTP requests to ScreenScraper.
- Transient failures (network errors, timeouts, 429, 5xx, an overloaded server) are retried up to 3 times with 2 s and 4 s backoff, plus 10 s after a 429.
- Permanent failures stop immediately with the reason, e.g. bad developer credentials (403), API closed (423) or daily quota exceeded (430).
- Media downloads go through ScreenScraper too and count against your quota. Check it with `scrapegoat user-info`.

## Troubleshooting

**`game not found in ScreenScraper database`**
- The dump may not be in ScreenScraper's database; try another dump or version.
- Check the platform with `scrapegoat list-platforms`.
- The "not found" result is cached for `--cache-hours`. Lower that value to retry sooner.

**`rate limited by API` or a quota error**
- Check your quota with `scrapegoat user-info`. Quotas reset daily.
- Lower `--workers`, since media downloads also use quota.

**`Failed to hash ROM`**
- Check file permissions and make sure the file isn't corrupted.
- For ZIP files, make sure the archive contains a ROM.

**High API usage**
- Run with `--verbose` to see every request and cache hit.
- Keep the same `--cache-dir` between runs so cached lookups and media are reused.

## Contributing

Pull requests are welcome. Run `make all` (fmt, vet, test, build) before submitting. Issues are tracked at https://github.com/tedkulp/scrapegoat/issues.

## Credits

- [ScreenScraper.fr](https://www.screenscraper.fr/) for the API and database
- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
- Made for [EmulationStation](https://github.com/RetroPie/EmulationStation)

## License

MIT. See [LICENSE](LICENSE).
