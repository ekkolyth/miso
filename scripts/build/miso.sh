BINARY=${BINARY:-miso}
PKG=./apps/miso/cmd

mkdir -p apps/miso/bin
cd apps/miso && go build -ldflags "-X github.com/ekkolyth/miso/internal/cli/commands.Version=dev" -o bin/$BINARY ./cmd
echo "miso built successfully"
