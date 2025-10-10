# Quick Start Guide

Get up and running with Scrapegoat in 5 minutes!

## 1. Get ScreenScraper Credentials

1. Register at https://www.screenscraper.fr/
2. Note your username and password
3. Contact ScreenScraper support or check their forums for developer credentials (devid and devpassword)

## 2. Configure Scrapegoat

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

## 3. Run Your First Scrape

```bash
# Dry run to test (doesn't download anything)
./scrapegoat --platform nes --rom-dir ~/roms/nes --dry-run

# Real scrape
./scrapegoat --platform nes --rom-dir ~/roms/nes --verbose
```

## 4. Results

After scraping, you'll have:
- `gamelist.xml` - Game metadata for EmulationStation
- `media/` - Downloaded images and videos

## Common Platforms

Platform codes you can use with `--platform`:

- `nes` - Nintendo Entertainment System
- `snes` - Super Nintendo
- `genesis` - Sega Genesis
- `n64` - Nintendo 64
- `ps1` - PlayStation 1
- `gba` - Game Boy Advance
- `arcade` - Arcade

Add more platforms to `config.yaml` as needed!

## Tips

1. **Start small**: Test with a few ROMs first
2. **Use --dry-run**: Always test before downloading
3. **Be patient**: API calls are rate-limited (1 second delay between ROMs)
4. **Check logs**: Use `--verbose` to see what's happening

## Next Steps

- Read the full [README.md](README.md) for all options
- Add more platforms to your config
- Configure EmulationStation to use your gamelist.xml

## Troubleshooting

**Error: "screenscraper.dev_id is required"**
- Make sure you've edited `config.yaml` with your credentials

**Error: "platform X not found"**
- Check that the platform name matches your `config.yaml`
- Platform names are case-sensitive

**No games found**
- Verify ROM file extensions match those in `config.yaml`
- Check that your ROM directory path is correct

For more help, see the [README.md](README.md) troubleshooting section.
