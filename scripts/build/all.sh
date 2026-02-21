BINARY=${BINARY:-miso}

mkdir -p apps/miso/bin
cd apps/miso && GOOS=darwin GOARCH=amd64 go build -o bin/$BINARY-darwin-amd64 ./cmd
cd apps/miso && GOOS=darwin GOARCH=amd64 go build -o bin/misox-darwin-amd64 ./cmd
cd apps/miso && GOOS=darwin GOARCH=arm64 go build -o bin/$BINARY-darwin-arm64 ./cmd
cd apps/miso && GOOS=darwin GOARCH=arm64 go build -o bin/misox-darwin-arm64 ./cmd
cd apps/miso && GOOS=linux GOARCH=amd64 go build -o bin/$BINARY-linux-amd64 ./cmd
cd apps/miso && GOOS=linux GOARCH=amd64 go build -o bin/misox-linux-amd64 ./cmd
cd apps/miso && GOOS=linux GOARCH=arm64 go build -o bin/$BINARY-linux-arm64 ./cmd
cd apps/miso && GOOS=linux GOARCH=arm64 go build -o bin/misox-linux-arm64 ./cmd
cd apps/miso && GOOS=windows GOARCH=amd64 go build -o bin/$BINARY-windows-amd64.exe ./cmd
cd apps/miso && GOOS=windows GOARCH=amd64 go build -o bin/misox-windows-amd64.exe ./cmd
