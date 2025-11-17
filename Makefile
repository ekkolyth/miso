.PHONY: build install uninstall run test tidy fmt clean build-all publish publish.patch publish.minor publish.major publish.current npm.pack npm.test sync-package.json

DRY_RUN ?= 0

BINARY ?= miso
PKG    := ./cmd

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	@mkdir -p bin
	go build -o bin/$(BINARY) $(PKG)

build-all:
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 $(PKG)
	GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 $(PKG)
	GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 $(PKG)
	GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 $(PKG)
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe $(PKG)

install: build
	@echo "Installing $(BINARY) to $(GOBIN)"
	@cp bin/$(BINARY) $(GOBIN)/$(BINARY)
	@echo "✓ Installed $(BINARY) to $(GOBIN)"

uninstall:
	rm -f $(GOBIN)/$(BINARY)

go: build
	go run $(PKG) $(ARGS)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -w $$(go list -f '{{.Dir}}' ./...)

clean:
	rm -rf bin
	rm -f package.json
	rm -f miso-*.tgz

# Bump the version in package.json and, for non-dry runs, commit the change,
# create a git tag, and push. Default bump level is "patch".
publish: publish.patch

publish.patch:
	$(MAKE) _publish LEVEL=patch

publish.minor:
	$(MAKE) _publish LEVEL=minor

publish.major:
	$(MAKE) _publish LEVEL=major

# Publish the current version from package.json without bumping it.
# For non-dry runs, creates a git tag and pushes to origin.
publish.current:
	@if [ "$$(git status --porcelain)" != "" ]; then \
		echo "Working tree is not clean. Commit or stash changes before publishing."; \
		exit 1; \
	fi
	@if [ "$(DRY_RUN)" = "1" ]; then \
		CURRENT_VERSION=$$(go run ./.github/release/version); \
		echo "DRY RUN: current version is $$CURRENT_VERSION"; \
		echo "DRY RUN: would create tag v$$CURRENT_VERSION and push to origin"; \
	else \
		CURRENT_VERSION=$$(go run ./.github/release/version); \
		echo "Publishing current version $$CURRENT_VERSION"; \
		cp .github/package.json package.json; \
		if [ -n "$$(git status --porcelain package.json)" ]; then \
			git add package.json; \
			git commit -m "chore: sync package.json to root"; \
		fi; \
		if git rev-parse "v$$CURRENT_VERSION" >/dev/null 2>&1; then \
			echo "Tag v$$CURRENT_VERSION already exists locally, deleting it"; \
			git tag -d "v$$CURRENT_VERSION"; \
		fi; \
		git tag -a "v$$CURRENT_VERSION" -m "Release v$$CURRENT_VERSION"; \
		git push origin HEAD; \
		git push origin "v$$CURRENT_VERSION"; \
	fi

_publish:
	@if [ "$$(git status --porcelain)" != "" ]; then \
		echo "Working tree is not clean. Commit or stash changes before publishing."; \
		exit 1; \
	fi
	@if [ "$(LEVEL)" = "current" ]; then \
		$(MAKE) publish.current; \
	else \
		if [ "$(DRY_RUN)" = "1" ]; then \
			NEXT_VERSION=$$(go run ./.github/release/bump -level=$(LEVEL) -dry-run); \
			echo "DRY RUN: next version would be $$NEXT_VERSION"; \
			echo "DRY RUN: would create tag v$$NEXT_VERSION and push to origin"; \
		else \
			NEXT_VERSION=$$(go run ./.github/release/bump -level=$(LEVEL)); \
			echo "Bumped version to $$NEXT_VERSION"; \
			cp .github/package.json package.json; \
			git add .github/package.json package.json; \
			git commit -m "chore: release v$$NEXT_VERSION"; \
			git tag -a "v$$NEXT_VERSION" -m "Release v$$NEXT_VERSION"; \
			git push origin HEAD; \
			git push origin "v$$NEXT_VERSION"; \
		fi \
	fi

# Copy package.json and create a tarball to inspect what would be published
npm.pack:
	@echo "Copying package.json..."
	@cp .github/package.json package.json
	@echo "Creating npm pack tarball..."
	@npm pack --dry-run
	@echo "✓ Package tarball created. Inspect the output above."
	@echo "To see the actual tarball contents, run: npm pack && tar -tzf miso-*.tgz"

# Test npm publish with --dry-run flag (shows what would be published without actually publishing)
npm.test:
	@echo "Testing npm publish with --dry-run..."
	@cp .github/package.json package.json
	@npm publish --dry-run
	@echo "✓ Dry run complete. No actual publish was performed."

# Sync package.json from .github/package.json to root
sync-package.json:
	@echo "Syncing package.json from .github/package.json..."
	@cp .github/package.json package.json
	@echo "✓ package.json synced to root"
