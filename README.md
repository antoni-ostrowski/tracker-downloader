# Usage

```bash
 ./tracker-downloader output-dir-path tracker-csv-path worker-count album-covers-dir-path

 # example
 # keep the worker count under 20
 ./tracker-downloader /Users/antoni-ostrowski/Desktop/new-music /Users/antoni-ostrowski/Desktop/yeat-tracker.csv 20 ./covers/

```

Script parses csv, downloads the files (coverts mp4s to mp3s if needed) and attaches metadata like album cover or album. If file already exists, it gets skipped.
