# Biome lint on docs
(cd apps/docs && npx biome lint .)

# golangci-lint on Go app
(cd apps/miso && golangci-lint run)
