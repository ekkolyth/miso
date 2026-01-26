.PHONY: build install uninstall run test tidy fmt clean build-all publish publish.patch publish.minor publish.major publish.current npm.pack npm.test

DRY_RUN ?= 0

BINARY ?= miso
PKG    := ./apps/miso/cmd

GOBIN ?= $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

build:
	@mkdir -p apps/miso/bin
	cd apps/miso && go build -o bin/$(BINARY) ./cmd

build-all:
	@mkdir -p apps/miso/bin
	cd apps/miso && GOOS=darwin GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 ./cmd
	cd apps/miso && GOOS=darwin GOARCH=amd64 go build -o bin/misox-darwin-amd64 ./cmd
	cd apps/miso && GOOS=darwin GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 ./cmd
	cd apps/miso && GOOS=darwin GOARCH=arm64 go build -o bin/misox-darwin-arm64 ./cmd
	cd apps/miso && GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 ./cmd
	cd apps/miso && GOOS=linux GOARCH=amd64 go build -o bin/misox-linux-amd64 ./cmd
	cd apps/miso && GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 ./cmd
	cd apps/miso && GOOS=linux GOARCH=arm64 go build -o bin/misox-linux-arm64 ./cmd
	cd apps/miso && GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe ./cmd
	cd apps/miso && GOOS=windows GOARCH=amd64 go build -o bin/misox-windows-amd64.exe ./cmd

install: build
	@echo "Installing $(BINARY) to $(GOBIN)"
	@cp apps/miso/bin/$(BINARY) $(GOBIN)/$(BINARY)
	@cp apps/miso/bin/$(BINARY) $(GOBIN)/misox
	@echo "✓ Installed $(BINARY) and misox to $(GOBIN)"

uninstall:
	rm -f $(GOBIN)/$(BINARY)
	rm -f $(GOBIN)/misox

go: build
	cd apps/miso && go run ./cmd $(ARGS)

test:
	cd apps/miso && go test ./...

tidy:
	cd apps/miso && go mod tidy

fmt:
	cd apps/miso && gofmt -w $$(go list -f '{{.Dir}}' ./...)

clean:
	rm -rf apps/miso/bin
	rm -f apps/miso/miso-*.tgz

# Bump the version in package.json and, for non-dry runs, commit the change,
# create a git tag, and push. Default bump level is "patch".
# Usage: make publish.patch MESSAGE="release message" or make publish.patch fixed bug
# Example: make publish.patch MESSAGE="fixed bug"
# Example: make publish.patch fixed bug
publish: publish.patch

# Helper to capture trailing arguments as MESSAGE
# This allows: make publish.patch fixed bug
# Also supports: make publish.patch MESSAGE="fixed bug"
publish.patch:
	@$(eval ARGS := $(filter-out $@ publish.patch publish.minor publish.major publish.current build install uninstall run test tidy fmt clean build-all npm.pack npm.test go,$(MAKECMDGOALS)))
	@$(if $(MESSAGE),,$(eval MESSAGE := $(ARGS)))
	@$(MAKE) _publish LEVEL=patch MESSAGE="$(MESSAGE)"

publish.minor:
	@$(eval ARGS := $(filter-out $@ publish.patch publish.minor publish.major publish.current build install uninstall run test tidy fmt clean build-all npm.pack npm.test go,$(MAKECMDGOALS)))
	@$(if $(MESSAGE),,$(eval MESSAGE := $(ARGS)))
	@$(MAKE) _publish LEVEL=minor MESSAGE="$(MESSAGE)"

publish.major:
	@$(eval ARGS := $(filter-out $@ publish.patch publish.minor publish.major publish.current build install uninstall run test tidy fmt clean build-all npm.pack npm.test go,$(MAKECMDGOALS)))
	@$(if $(MESSAGE),,$(eval MESSAGE := $(ARGS)))
	@$(MAKE) _publish LEVEL=major MESSAGE="$(MESSAGE)"

# Publish the current version from package.json without bumping it.
# For non-dry runs, creates a git tag and pushes to origin.
publish.current:
	@CURRENT_BRANCH=$$(git branch --show-current); \
	if [ "$$CURRENT_BRANCH" = "dev" ]; then \
		echo "Error: Cannot publish from dev branch. Please switch to main or another branch."; \
		exit 1; \
	fi
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
		if git rev-parse "v$$CURRENT_VERSION" >/dev/null 2>&1; then \
			echo "Tag v$$CURRENT_VERSION already exists locally, deleting it"; \
			git tag -d "v$$CURRENT_VERSION"; \
		fi; \
		git tag -a "v$$CURRENT_VERSION" -m "Release v$$CURRENT_VERSION"; \
		git push origin HEAD; \
		git push origin "v$$CURRENT_VERSION"; \
	fi

_publish:
	@CURRENT_BRANCH=$$(git branch --show-current); \
	if [ "$$CURRENT_BRANCH" = "dev" ]; then \
		echo "Error: Cannot publish from dev branch. Please switch to main or another branch."; \
		exit 1; \
	fi
	@if [ "$$(git status --porcelain)" != "" ]; then \
		echo "Working tree is not clean. Commit or stash changes before publishing."; \
		exit 1; \
	fi
	@if [ "$(LEVEL)" = "current" ]; then \
		$(MAKE) publish.current; \
	else \
		if [ "$(DRY_RUN)" = "1" ]; then \
			NEXT_VERSION=$$(go run ./.github/release/bump -level=$(LEVEL) -dry-run); \
			if [ -n "$(MESSAGE)" ]; then \
				echo "DRY RUN: next version would be $$NEXT_VERSION"; \
				echo "DRY RUN: would create tag v$$NEXT_VERSION with message: release v$$NEXT_VERSION: $(MESSAGE)"; \
			else \
				echo "DRY RUN: next version would be $$NEXT_VERSION"; \
				echo "DRY RUN: would create tag v$$NEXT_VERSION and push to origin"; \
			fi; \
		else \
			NEXT_VERSION=$$(go run ./.github/release/bump -level=$(LEVEL)); \
			echo "Bumped version to $$NEXT_VERSION"; \
			git add apps/miso/package.json; \
			git commit -m "chore: release v$$NEXT_VERSION"; \
			if [ -n "$(MESSAGE)" ]; then \
				git tag -a "v$$NEXT_VERSION" -m "release v$$NEXT_VERSION: $(MESSAGE)"; \
			else \
				git tag -a "v$$NEXT_VERSION" -m "Release v$$NEXT_VERSION"; \
			fi; \
			git push origin HEAD; \
			git push origin "v$$NEXT_VERSION"; \
		fi \
	fi

# Create a tarball to inspect what would be published
npm.pack:
	@echo "Creating npm pack tarball..."
	@cd apps/miso && npm pack --dry-run
	@echo "✓ Package tarball created. Inspect the output above."
	@echo "To see the actual tarball contents, run: cd apps/miso && npm pack && tar -tzf miso-*.tgz"

# Test npm publish with --dry-run flag (shows what would be published without actually publishing)
npm.test:
	@echo "Testing npm publish with --dry-run..."
	@cd apps/miso && npm publish --dry-run
	@echo "✓ Dry run complete. No actual publish was performed."

# Catch-all to prevent make from complaining about unknown targets
# This allows trailing arguments to be passed through (e.g., make publish.patch fixed bug)
%:
	@:
