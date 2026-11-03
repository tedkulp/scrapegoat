# Quick Start Guide

Get up and running with Scrapegoat in 5 minutes.

## 1. Get ScreenScraper Credentials

1. Register at https://www.screenscraper.fr/
2. Note your username and password
3. Contact ScreenScraper support or check their forums for developer credentials (devid and devpassword)

## 2. Get Scrapegoat

Download the archive for your OS from [GitHub Releases](https://github.com/tedkulp/scrapegoat/releases) and extract `scrapegoat`, or build it from source:

```bash
git clone https://github.com/tedkulp/scrapegoat.git
cd scrapegoat
make build
```

## 3. Configure Scrapegoat

```bash
# Copy example config
cp config.yaml.example config.yaml

# Edit with your credentials
nano config.yaml  # or your favorite editor
```

Update these fields:
```yaml
screenscraper:
  dev_id: "YOUR_DEV_ID"
  dev_password: "YOUR_DEV_PASSWORD"
  user_id: "YOUR_USERNAME"
  user_password: "YOUR_PASSWORD"
```

Check that the credentials work:

```bash
./scrapegoat user-info
```

## 4. Run Your First Scrape

```bash
# Dry run to test (doesn't download or write anything)
./scrapegoat scrape --platform nes --rom-dir ~/roms/nes --dry-run

# Real scrape
./scrapegoat scrape --platform nes --rom-dir ~/roms/nes --verbose
```

> [!WARNING]
> At the end of a run, Scrapegoat lists the ROMs it could not match and **asks whether to delete them**. Add `--auto-delete=false` to keep them without being asked.

## 5. Results

After scraping, your ROM directory contains:
- `gamelist.xml` - Game metadata for EmulationStation
- Media subdirectories such as `box2dfront/`, `screenshots/`, `wheel/` and `videos/`, one file per ROM

Use `--gamelist-dir` and `--media-root-dir` to write these somewhere else.

## Common Platforms

Platform slugs you can use with `--platform`:

- `nes` - Nintendo Entertainment System
- `snes` - Super Nintendo
- `genesis` - Sega Genesis / Mega Drive
- `n64` - Nintendo 64
- `psx` - PlayStation
- `gb` / `gbc` / `gba` - Game Boy, Game Boy Color, Game Boy Advance

Platforms are fetched from ScreenScraper automatically and cached in `~/.scrapegoat/platforms.json`. Run `./scrapegoat list-platforms` to see every slug; slugs are not case-sensitive, and most platforms have several (one per name ScreenScraper gives it), so `famicom` picks the same platform as `nes`.

## Tips

1. **Start small**: Test with a few ROMs first
2. **Use --dry-run**: Always test before downloading
3. **Be patient**: Requests are throttled (at least 1 second between requests; cached lookups skip the wait)
4. **Check logs**: Use `--verbose` to see what's happening

## Next Steps

- Read the full [README.md](README.md) for all options
- Configure EmulationStation to use your gamelist.xml
- Try custom artwork generation: [ARTWORK.md](ARTWORK.md)

## Troubleshooting

**Error: "screenscraper.dev_id is required"**
- Make sure you've edited `config.yaml` with your credentials
- Scrapegoat looks for `./config.yaml`, then `~/.config/scrapegoat/config.yaml`; use `--config` to point elsewhere

**Error: "platform X not found in configuration or API"**
- Check the slug with `./scrapegoat list-platforms`

**No games found**
- Run `./scrapegoat list-platforms` and check that your ROM file extensions are listed for the platform
- Check that your ROM directory path is correct

For more help, see the [README.md](README.md) troubleshooting section.
