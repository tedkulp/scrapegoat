# The media cache is keyed by ScreenScraper system ID, and old slug folders are moved once

Media is cached at `<cache-dir>/cache/<system-id>/<media-type>/<rom-name>.<ext>`, not under the `--platform` slug as typed. This replaces the `<platform>` part of the path in ADR 0001; the sidecar decision there stands. Since the platform catalog (#8), `nes`, `famicom` and `NES` all pick platform 3, and keying the cache by the typed slug gave each its own copy, so the same media was downloaded again against the daily quota. The system ID is assigned by ScreenScraper and does not change when slug or alias rules do.

At the start of a scrape (not a dry run), `media.MigrateLegacyCache` moves `cache/<typed slug>/` into `cache/<system-id>/`. A file the ID folder already has wins, and the legacy copy and its sidecar are dropped; the legacy folder is then removed. Sidecars record the system, game and media IDs, not the folder, so moved files still match.

## Considered Options

- **Key by a canonical slug**: rejected. Slugs are worked out from ScreenScraper's names and our alias table, so a later rule change would move the cache again.
- **Re-key without moving old folders**: rejected. Users hold gigabytes of cached media that would cost daily quota to fetch again.
- **Read old slug folders as a fallback forever**: rejected, for the same reason ADR 0001 rejected two layouts.

## Consequences

- Only the folder for the slug a run is given is moved. A cache under another slug for the same platform moves the first time that slug is used.
- A dry run before the first real run reports cached media as downloads, because it does not move the folder.
- `cache/<system-id>` is less readable than a slug when browsing the cache by hand.
