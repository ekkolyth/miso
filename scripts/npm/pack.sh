echo "Creating npm pack tarball..."
cd apps/miso && npm pack --dry-run
echo "✓ Package tarball created. Inspect the output above."
echo "To see the actual tarball contents, run: cd apps/miso && npm pack && tar -tzf miso-*.tgz"
