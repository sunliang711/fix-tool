APP := fix-tool
CMD := ./cmd/fix-tool
DIST_DIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
TARGETS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
LD_FLAGS := -s -w -X 'fix-tool/internal/version.Version=$(VERSION)' -X 'fix-tool/internal/version.Commit=$(COMMIT)' -X 'fix-tool/internal/version.BuildTime=$(BUILD_TIME)'

.PHONY: build clean cross-build release test vuln

build:
	mkdir -p $(DIST_DIR)/bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LD_FLAGS)" -o $(DIST_DIR)/bin/$(APP) $(CMD)

clean:
	rm -rf $(DIST_DIR)

test:
	go test ./... -count=1

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

cross-build:
	rm -rf $(DIST_DIR)/cross
	mkdir -p $(DIST_DIR)/cross
	for target in $(TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		out="$(DIST_DIR)/cross/$(APP)_$${os}_$${arch}"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LD_FLAGS)" -o "$$out" $(CMD); \
	done

release:
	rm -rf $(DIST_DIR)/release
	mkdir -p $(DIST_DIR)/release
	for target in $(TARGETS); do \
		os=$${target%/*}; \
		arch=$${target#*/}; \
		pkg="$(APP)_$(VERSION)_$${os}_$${arch}"; \
		pkg_dir="$(DIST_DIR)/release/$$pkg"; \
		mkdir -p "$$pkg_dir/docs/project/fix-tool" "$$pkg_dir/testdata/scenarios" "$$pkg_dir/testdata/dictionaries"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LD_FLAGS)" -o "$$pkg_dir/$(APP)" $(CMD); \
		cp README.md "$$pkg_dir/README.md"; \
		cp config-example.toml "$$pkg_dir/config-example.toml"; \
		cp docs/project/fix-tool/user-guide.md "$$pkg_dir/docs/project/fix-tool/user-guide.md"; \
		cp docs/project/fix-tool/faq.md "$$pkg_dir/docs/project/fix-tool/faq.md"; \
		cp testdata/scenarios/order-lifecycle.yaml "$$pkg_dir/testdata/scenarios/order-lifecycle.yaml"; \
		cp testdata/dictionaries/custom-tags.toml "$$pkg_dir/testdata/dictionaries/custom-tags.toml"; \
		tar -C "$(DIST_DIR)/release" -czf "$(DIST_DIR)/release/$$pkg.tar.gz" "$$pkg"; \
		rm -rf "$$pkg_dir"; \
	done
	cd $(DIST_DIR)/release && shasum -a 256 *.tar.gz > checksums.txt
