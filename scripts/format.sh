# Biome format on docs
(cd apps/docs && npx biome format --write .)

# Go fmt on Go app
(cd apps/miso && go fmt ./...)
