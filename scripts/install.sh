BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN)
[ -z "$GOBIN" ] && GOBIN=$(go env GOPATH)/bin

# Install JS deps for all workspaces (apps/docs etc.) from repo root
echo "→ installing dependencies..."
bun install
echo "✓ dependencies installed"

# Build first
./scripts/build/miso.sh

echo "Installing $BINARY to $GOBIN"
cp apps/miso/bin/$BINARY $GOBIN/$BINARY || exit 1
cp apps/miso/bin/$BINARY $GOBIN/misox || exit 1
echo "✓ Installed $BINARY and misox to $GOBIN"
