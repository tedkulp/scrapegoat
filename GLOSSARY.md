# Scrapegoat

Scrapegoat identifies ROM files against the ScreenScraper database and turns what it finds into EmulationStation metadata and media.

## Language

### Scraping a ROM

**Scrape**:
The whole job of turning one ROM into EmulationStation metadata and media: hashing it, looking it up, placing its media and composing its artwork.
_Avoid_: Process, lookup (a lookup is one step of a scrape)

**Scrape outcome**:
What a scrape produces: scraped, not found, or non-game. A scrape that could not hash or look up the ROM is a failure, not an outcome; one stopped by a cutoff leaves a deferred ROM, which is neither.
_Avoid_: Status, result type

**Scraped**:
A scrape outcome where the ROM matched a game and its media and artwork were placed (or, in a dry run, planned).
_Avoid_: Success, done

**New media**:
The run summary's count of scraped ROMs that got at least one media file newly placed in the media root: downloaded or copied from the media cache (in a dry run, planned that way). Files already in the media root, failed files and artwork do not count. With `--force-redownload` every re-placed file counts.
_Avoid_: Downloaded ROMs (cache copies count too)

**Unmatched ROM**:
A ROM whose scrape ended as not found, non-game or a failure other than a cutoff. Unmatched ROMs are what Scrapegoat offers to delete at the end of a run.
_Avoid_: Failed ROM (not found and non-game ROMs did not fail), bad ROM

**Deferred ROM**:
A ROM the run could not look up because of a cutoff. A deferred ROM is left for the next run: it is never unmatched and never offered for deletion.
_Avoid_: Skipped ROM (skipped suggests non-game), failed ROM, unmatched ROM

**Unmatched ROM disposition**:
What happens to a run's unmatched ROMs once the run ends: keep them, delete them, or ask the user.
_Avoid_: Cleanup policy, auto-delete (that is the flag that chooses the disposition)

### Looking up a ROM

**Lookup**:
Asking ScreenScraper (or the local cache of its earlier answers) which game a ROM is, identified by its hashes, size and filename on a given system.
_Avoid_: Query, scrape (scrape is the whole per-ROM job, not just this step)

**Lookup outcome**:
The answer a lookup produces: a match, not found, or non-game. A lookup that could not get an answer at all is a failure, not an outcome. After a cutoff, an answer cached by an earlier run still counts: a match however old, a not found or non-game only while it is within the cache lifetime. An expired not found or non-game makes the ROM a deferred ROM, so a ROM ScreenScraper may have added since is never deleted.
_Avoid_: Result status, response

**Match**:
A lookup outcome where ScreenScraper identified the ROM as a game.
_Avoid_: Hit, found

**Not found**:
A lookup outcome where ScreenScraper has no entry for the ROM.
_Avoid_: Miss, unknown

**Non-game**:
A lookup outcome where ScreenScraper identifies the ROM but classifies it as something other than a game, such as a BIOS, demo or utility.
_Avoid_: Invalid game, skipped game

### Talking to ScreenScraper

**Quota**:
The number of ScreenScraper requests the user account may make per day, and how many it has already used today.
_Avoid_: Rate limit (that is the server refusing a request), allowance

**Failed-request quota**:
A separate daily allowance of ScreenScraper requests that may fail (ScreenScraper's "KO" fields, `requestskotoday` and `maxrequestskoperday`), counted apart from the main quota.
_Avoid_: KO quota, error limit

**Cutoff**:
ScreenScraper refusing every further request from a run, because the quota or failed-request quota is used up, the developer credentials are rejected, the software is blacklisted or the API is closed. After a cutoff, the run sends no more requests, for lookups or media.
_Avoid_: Rate limit (one refused request), quota exceeded (one cause of a cutoff), halt

**Throttle**:
The minimum gap Scrapegoat leaves between consecutive ScreenScraper requests, so it stays polite and inside its per-minute request limit.
_Avoid_: Delay, sleep, backoff (backoff is the wait before a retry)

### Placing media

**Media type**:
A kind of media EmulationStation shows for a game, such as box front, screenshot or video. Each ROM gets at most one file per media type.
_Avoid_: Asset type, image kind

**Media root**:
The directory EmulationStation reads media from, with one subdirectory per media type.
_Avoid_: Final location, output dir, destination

**Media cache**:
Scrapegoat's own copy of every media file it has fetched from ScreenScraper, kept across runs so placing the same media again costs no quota.
_Avoid_: Temp dir, download dir

**Media identity**:
What a media file is in ScreenScraper's terms: the system, the game and the media name with its region. It tells whether a cached file is still the one a lookup now selects.
_Avoid_: Media URL (the URL also carries credentials and a mirror host)

**Media plan**:
For one ROM, which media files are already in the media root, which come from the media cache, and which must be downloaded.
_Avoid_: Download list

### Composing artwork

**Artwork**:
An image Scrapegoat composes from a game's media, such as a screenshot framed with its box art and wheel. EmulationStation calls these miximages.
_Avoid_: Composite, generated media (media comes from ScreenScraper; artwork is made locally)

**Artwork output**:
One kind of artwork the user defines, which produces one image per ROM, all in the same folder.
_Avoid_: Output type, template

### Choosing a platform

**Platform**:
A game system ScreenScraper catalogs, such as the NES or the Atari ST, with its ID, names and ROM file extensions. Every scrape runs against one platform.
_Avoid_: System (ScreenScraper's word; use it only when talking about the API), console

**Platform slug**:
A short lowercase name the user types to pick a platform, made from one of the platform's names, such as `nes` or `atarist`. A platform with several names has several slugs.
_Avoid_: Platform name (the name is ScreenScraper's display text), platform key

**Platform alias**:
A slug Scrapegoat adds for a platform when none of its own names produce the one users expect, such as `atari2600`.
_Avoid_: Nickname, shortcut

**Platform catalog**:
Every platform Scrapegoat knows, from ScreenScraper and from the user's configuration, together with the slugs and aliases that pick them.
_Avoid_: Platform list, platform cache (the cache is where the catalog is kept between runs)
