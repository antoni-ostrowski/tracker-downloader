# Usage

```bash
./tracker-downloader -o ./output -i /Users/antoni-ostrowski/Desktop/yeat-tracker.csv -w 20 -c ./covers/
# or
just run
# dont get crazy with worker count (20 works okay)

```

Script parses csv, downloads the files (coverts mp4s to mp3s if needed) and attaches metadata like album cover or album. If file already exists, it gets skipped.
