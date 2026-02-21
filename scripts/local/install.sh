BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN)
[ -z "$GOBIN" ] && GOBIN=$(go env GOPATH)/bin

# Build first
./scripts/build/miso.sh

echo "Installing $BINARY to $GOBIN"
cp apps/miso/bin/$BINARY $GOBIN/$BINARY || exit 1
cp apps/miso/bin/$BINARY $GOBIN/misox || exit 1
echo "✓ Installed $BINARY and misox to $GOBIN"

# Install shell completions
node apps/miso/scripts/postinstall.mjs 2>/dev/null || true
