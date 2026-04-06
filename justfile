default:
  just --list
run:
  ./tracker-downloader -o ./output -i /Users/antoni-ostrowski/Desktop/yeat-tracker.csv -w 20 -c ./covers/ 

build:
  go build -o tracker-downloader ./cmd/main.go
