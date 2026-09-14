PLUGIN  := cantilune
DIST    := dist
VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//')
WEBSITE ?=
# Fixed file dates make the package reproducible: the same commit always gives the same .ndp.
SOURCE_DATE_EPOCH ?= $(shell git log -1 --format=%ct 2>/dev/null || echo 0)

.PHONY: all tidy generate test lint fuzz bench build package checksums clean

all: package

tidy:
	go mod tidy

# regenerate manifest.json from catalog/presets.json
generate:
	go generate ./...

test:
	go test ./...

lint:
	golangci-lint run ./...

# short fuzzing run of every fuzz target
fuzz:
	@for target in $$(grep -ho '^func Fuzz[A-Za-z]*' *_test.go | sed 's/func //'); do \
		go test -run '^$$' -fuzz "^$$target$$" -fuzztime 15s . || exit 1; \
	done

bench:
	go test -run '^$$' -bench . -benchtime 5x .
	CANTILUNE_LOAD=1 go test -run TestLoadProfile -v .

build:
	mkdir -p $(DIST)
	tinygo build -no-debug -o $(DIST)/plugin.wasm -target wasip1 -buildmode=c-shared .

# The file name determines the plugin ID in Navidrome, so it is always cantilune.ndp (without version).
package: build
	@if command -v jq >/dev/null 2>&1; then \
		jq --arg v "$(or $(VERSION),0.0.0-dev)" --arg w "$(WEBSITE)" \
			'.version = $$v | if $$w != "" then .website = $$w else . end' \
			manifest.json > $(DIST)/manifest.json; \
	else \
		cp manifest.json $(DIST)/manifest.json; \
	fi
	touch -d @$(SOURCE_DATE_EPOCH) $(DIST)/manifest.json $(DIST)/plugin.wasm
	cd $(DIST) && rm -f $(PLUGIN).ndp && TZ=UTC zip -X -D -q $(PLUGIN).ndp manifest.json plugin.wasm
	@echo "Package created: $(DIST)/$(PLUGIN).ndp"

checksums: package
	cd $(DIST) && sha256sum $(PLUGIN).ndp plugin.wasm > SHA256SUMS && cat SHA256SUMS

clean:
	rm -rf $(DIST)
