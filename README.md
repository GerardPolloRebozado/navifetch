# Navifetch

This tool is in the early stages of development. For now, it has only been tested with Navidrome and Aonsoku
Navifetch is a specialized proxy for Subsonic-compatible music servers (such as Navidrome). It enhances your music streaming experience by acting as an intermediary that can fetch missing content on the fly.

### How It Works

When a client makes a search request, Navifetch forwards the query to your Subsonic server. If the requested content is not found in your local library, Navifetch automatically searches the iTunes API. 

If you choose to play a song found via iTunes, Navifetch downloads it using `yt-dlp` and streams it to your client. 
- **Temporary Streaming**: Files downloaded for streaming are automatically deleted after 24 hours.
- **Persistent Downloads**: If you add a track to a playlist, it is downloaded.

### Disclaimer

This tool is intended for use with authorized content only. The developer is not responsible for copyright infringement or any damage resulting from the use of this software.

### Features

- Seamless integration with Subsonic-compatible clients.
- Automatic fallback to LastFM/iTunes/MusicBrainz API for missing content. (iTunes doesn't work very well)
- Dynamic downloading and streaming.
- Automatic cleanup of temporary files.
- Persistent storage for tracks is added to playlists.

### Installation

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
    restart: unless-stopped
    volumes:
      - /path/to/music:/music

```

### Configuration

Navifetch can be configured using the following environment variables:
It is recommended to use LastFM for metadata as it has a better search engine, also iTunes won't be able to stream directly when downloading a song it will only be able to download it and then search again to stream it.

| Variable            | Description                                                   | Default |
|---------------------|---------------------------------------------------------------|---------|
| `NAVIDROME_BASE`    | **Required**. The base URL of your Subsonic/Navidrome server. | None    |
| `COUNTRY`           | The country code to use for iTunes API requests.              | `US`    |
| `METADATA_PROVIDER` | The metadata provider to use: `itunes`, `musicbrainz`, or `lastfm`. | None    |
| `LASTFM_API_KEY`    | **Required for lastfm**. Your Last.fm API key.                 | None    |
| `RESULTS_PER_PAGE`  | The number of results to display per page.                    | `10`    |

