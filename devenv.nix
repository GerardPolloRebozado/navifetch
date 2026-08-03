{ pkgs, ... }:

{
  dotenv.enable = true;

  # https://devenv.sh/packages/
  packages = with pkgs; [
    go
    ffmpeg
    yt-dlp
    spotdl
    python3
  ];

  # https://devenv.sh/languages/
  languages.go.enable = true;

  # https://devenv.sh/scripts/
  scripts.run-navifetch.exec = "go run src/main.go";
  scripts.build-navifetch.exec = "go build -o bin/navifetch src/main.go";
}
