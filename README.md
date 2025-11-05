# jnovels-scrape

[![Build Status](https://ci.skobk.in/api/badges/skobkin/jnovel-scrape/status.svg)](https://ci.skobk.in/skobkin/jnovel-scrape)

`jnovels-scrape` is a Go CLI that gathers the latest posts from [jnovels.com](https://jnovels.com), normalises their metadata, and generates a Markdown table of releases. The tool prefers the WordPress REST API and transparently falls back to HTML scraping when the API is unavailable.

## Features

- WordPress REST API pagination with automatic stop once posts fall below the cutoff date
- HTML fallback that mirrors the site archive when the API cannot be reached
- Type (`EPUB`, `PDF`, `MANGA`, `UNKNOWN`) and volume inference with warnings on partial data
- Filters on type, title substring, and exact volume match
- Client rate limiting and respectful handling of server-side throttling (`Retry-After` / backoff)
- Markdown output sorted by date (desc) then title (asc)

## Install

```sh
go build ./cmd/jnovels-scrape
```

This produces the `jnovels-scrape` binary in the current directory.

## Usage

The cutoff date (`--until`) is required and uses `YYYY-MM-DD`.

```sh
# Simple run
./jnovels-scrape --until 2024-12-01 --out releases.md

# More advanced
./jnovels-scrape --until 2025-01-01 --group title --type EPUB --out result.md
```

Common flags:

| Flag | Description |
| --- | --- |
| `--until <date>` | Required cutoff date (`YYYY-MM-DD`). Only posts on/after this date are kept. |
| `--type, -t <list>` | Comma-separated subset of `epub,pdf,manga,unknown` (case-insensitive). |
| `--title, --name, -n <substr>` | Case-insensitive title substring filter. |
| `--volume, -v <num>` | Filter by exact volume (integer or decimal). Posts without a recognised volume are dropped. |
| `--out <path>` | Output file. Default is `stdout`. |
| `--max-pages <int>` | Safety limit when paging (default `2000`). |
| `--concurrency <int>` | Detail fetch concurrency for HTML fallback (default `4`). |
| `--req-interval <duration>` | Minimum interval between every HTTP request (default `600ms`). |
| `--limit-wait <duration>` | Wait time when the server rate limits without `Retry-After` (default `60s`). |
| `--group <none\|title>` | Group output rows before sorting (default `none`). |
| `--group-sort <asc\|desc>` | Sort order inside title groups when `--group=title` (default `asc`). |
| `--mode <auto\|api\|html>` | Select fetch strategy. `auto` (default) tries the API before falling back to HTML. |

### Example

```sh
./jnovels-scrape \
  --until 2024-11-01 \
  --type epub,pdf \
  --title "mercenary" \
  --req-interval 1s \
  --out mercenary.md
```

## Modes & Fallback

- **auto** (default): Try the WordPress REST API first; on failure, fall back to HTML crawling.
- **api**: Force API-only mode. The command exits with an error if the API is unreachable.
- **html**: Force HTML-only scraping (never hitting the API).

API mode uses `wp-json/wp/v2/posts` with `per_page=100`, `orderby=date`, and an `after` parameter derived from `--until`. Taxonomies are fetched once to improve type inference. HTML mode mirrors the `/page/{n}/` archives, extracts titles/links, and loads each post to read the authoritative publish date, categories, and tags.

Warnings are emitted for partial records (e.g., blank volumes, `UNKNOWN` type, skipped posts without publish dates). These appear on stderr prefixed with `WARN`.

## Rate Limiting

- **Client-side throttle (`--req-interval`)**: Enforced for every HTTP request including taxonomy lookups and post detail fetches.
- **Server-side limits (`--limit-wait`)**: Applied when a `429`/`503` response lacks a `Retry-After` header. When the header is present, the tool sleeps for the provided duration.

A small jitter (±10%) is added to retry waits to avoid thundering herds.

## Output format

The generated Markdown begins with a header noting the cutoff date followed by a table:

```
Generated from jnovels.com (cutoff: 2024-12-01)

| Title | Volume | Type | Date | Link |
|---|---:|---|---|---|
| Example Title | 4 | PDF | 2024-12-02 | [link](https://jnovels.com/example-title-volume-4-pdf/) |
```

Titles are HTML-stripped, entities are unescaped, pipes are escaped, and dates are normalised to `YYYY-MM-DD` (UTC). Volume cells may be blank when no numeric volume is present.

### Grouping

Use `--group=title` to cluster releases that share the same cleaned title (e.g. EPUB/PDF pairs or different volume parts). Title groups are sorted alphabetically, and the rows inside each group are ordered by volume number (`--group-sort=asc|desc`, default ascending). Entries without a parsed volume stay within their title group but follow the numbered volumes.

## Development

```sh
go test ./...
```

The repository contains unit tests for filters, type/volume parsing, and time parsing.

CI (GitHub Actions) runs formatting checks (`go fmt`), `go vet`, and the unit test suite on every push.
