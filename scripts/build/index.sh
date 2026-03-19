SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"

echo "→ building miso..."
cd apps/miso
mkdir -p bin
go build -o bin/miso ./cmd
echo "✓ miso built"

cd "$REPO_ROOT"

echo "→ building docs..."
cd apps/docs && bun run build
echo "✓ docs built"
