# Biome lint on docs
(cd apps/docs && npx biome lint .)

# Go vet on Go app
(cd apps/miso && go vet ./...)
