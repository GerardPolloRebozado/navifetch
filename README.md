# Navifetch

This tool is in the early stages of development. For now, it has only been tested with Navidrome and Aonsoku
Navifetch is a specialized proxy for Subsonic-compatible music servers (such as Navidrome). It enhances your music streaming experience by acting as an intermediary that can fetch missing content on the fly.

### How It Works


When a client makes a search request, Navifetch forwards the query to your Navidrome server. If the requested content is not found in your local library, Navifetch automatically searches the son with iTunes API and yt-dlp and returns the list of songs available externally, if the user plays one of that songs the song will be downloaded using yt-dlp and played, you can also add it to a playlist. After 24hrs if the song was only played but not added to a playlist the song will be removed to keep the files clean.

### Disclaimer

This tool is intended for use with authorized content only. The developer is not responsible for copyright infringement or any damage resulting from the use of this software.

### Installation & Development

#### Using devenv (Nix Environment)

If you use `devenv`, all dependencies (`go`, `ffmpeg`, `yt-dlp`, `spotdl`, `python3`) are automatically managed:

```bash
devenv shell
go run src/main.go
```

#### Docker Compose (Recommended)

Navifetch is available as a Docker image hosted on the GitHub Container Registry (GHCR). Use the following `docker-compose.yml` to deploy it:

```yaml
services:
  navifetch:
    image: ghcr.io/gerardpollorebozado/navifetch:latest
    container_name: navifetch
    ports:
      - "8080:8080"
    environment:
      - NAVIDROME_BASE=http://navidrome:4533
      - NAVIDROME_DB_PATH=/navidrome-data/navidrome.db
      - METADATA_PROVIDER=aggregator
    restart: unless-stopped
    volumes:
      - /path/to/music:/music
      - /path/to/navidrome_data:/navidrome-data:ro
```

### Configuration

| Variable            | Description                                                            | Default      |
|---------------------|------------------------------------------------------------------------|--------------|
| `NAVIDROME_BASE`    | **Required**. The base URL of your Subsonic/Navidrome server.          | None         |
| `NAVIDROME_DB_PATH` | Path to `navidrome.db` for instant & exact path-based song ID resolution after downloads. Mount Navidrome's data folder read-only (`:ro`). | None |
| `COUNTRY`           | The country code to use for iTunes API requests.                       | `US`         |
| `METADATA_PROVIDER` | The metadata provider to use: `aggregator` (default), `itunes`, or `musicbrainz`. | `aggregator` |
| `RESULTS_PER_PAGE`  | The number of results to display per page.                             | `10`         |
