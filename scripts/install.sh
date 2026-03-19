SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$REPO_ROOT"

echo "→ installing go dependencies..."
cd apps/miso && go install ./...
echo "✓ go dependencies installed"

cd "$REPO_ROOT"

echo "→ installing docs dependencies..."
cd apps/docs && bun install
echo "✓ docs dependencies installed"
