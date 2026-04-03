BINARY=${BINARY:-miso}
PKG=./apps/miso/cmd

mkdir -p apps/miso/bin
cd apps/miso && go build -o bin/$BINARY ./cmd
echo "miso built successfully"
