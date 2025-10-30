.PHONY: build clean

build:
	@mkdir -p bin
	go build -o bin/miso ./cmd

clean:
	rm -rf bin

install:
	go install ./cmd