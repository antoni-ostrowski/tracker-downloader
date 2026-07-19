default:
  just --list
dev:
  go run ./cmd/main.go -o ./output -i /Users/antoni-ostrowski/Downloads/yeat-tracker.csv -w 20 -c ./covers/ 

run:
  ./tracker-downloader -o ./output -i /Users/antoni-ostrowski/Downloads/yeat-tracker.csv -w 20 -c ./covers/ 

build:
  go build -o tracker-downloader ./cmd/main.go

clean:
  rm -rf ./output
