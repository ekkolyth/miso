BINARY=${BINARY:-miso}
GOBIN=$(go env GOBIN)
[ -z "$GOBIN" ] && GOBIN=$(go env GOPATH)/bin

# Build first
sh ./scripts/build/miso.sh

echo "Installing $BINARY to $GOBIN"
mkdir -p "$GOBIN"

# Stage in $GOBIN then rename. Writing over the destination in place reuses the
# inode, which invalidates the kernel's cached ad-hoc signature and gets the
# binary SIGKILLed on next exec (and miso is usually running this script).
install_atomic() {
    src=$1
    dest=$2
    tmp=$(mktemp "$(dirname "$dest")/.miso.XXXXXX") || exit 1
    cp "$src" "$tmp" || { rm -f "$tmp"; exit 1; }
    chmod 755 "$tmp" || { rm -f "$tmp"; exit 1; }
    mv -f "$tmp" "$dest" || { rm -f "$tmp"; exit 1; }
}

install_atomic apps/miso/bin/$BINARY "$GOBIN/$BINARY"
install_atomic apps/miso/bin/$BINARY "$GOBIN/misox"
echo "✓ Installed $BINARY and misox to $GOBIN"

# Install shell completions
node apps/miso/scripts/postinstall.mjs 2>/dev/null || true
