# Scrapegoat

Scrapegoat identifies ROM files against the ScreenScraper database and turns what it finds into EmulationStation metadata and media.

## Language

### Looking up a ROM

**Lookup**:
Asking ScreenScraper (or the local cache of its earlier answers) which game a ROM is, identified by its hashes, size and filename on a given system.
_Avoid_: Query, scrape (scrape is the whole per-ROM job, not just this step)

**Lookup outcome**:
The answer a lookup produces: a match, not found, or non-game. A lookup that could not get an answer at all is a failure, not an outcome.
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
