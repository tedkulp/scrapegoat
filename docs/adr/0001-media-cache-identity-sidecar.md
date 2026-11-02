# Media cache files carry a media identity sidecar instead of being keyed by URL

The media cache stays at `<cache-dir>/cache/<platform>/<media-type>/<rom-name>.<ext>`, and each cached file gets a sidecar recording its media identity (ScreenScraper's `systemeid`, `jeuid` and `media` URL parameters). A cached file whose sidecar disagrees with what a lookup now selects is downloaded again; a file with no sidecar is trusted and adopted. We did this because users already hold gigabytes of cached media that would cost daily quota to fetch again, and because the media URL itself can't be the key: it carries the developer and user passwords and changes with the ScreenScraper mirror host.

## Considered Options

- **Re-key the cache by a hash of the identity, reading old paths as a fallback**: rejected. It splits the cache into two layouts to maintain indefinitely, for no gain over a sidecar.
- **Leave the cache keyed by ROM name and rely on `--force-redownload`**: rejected. A region or game change would keep serving the wrong file without any sign.

## Consequences

- A cached file already stale before sidecars existed is adopted as correct. Only `--force-redownload` replaces it.
- The media root keeps an existence-only check and has no sidecars, so a stale file there is replaced only by `--force-redownload`.
