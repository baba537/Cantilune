PLUGIN  := cantilune
DIST    := dist
VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//')
WEBSITE ?=

.PHONY: all tidy generate test build package clean

all: package

tidy:
	go mod tidy

# manifest.json aus catalog/presets.json neu erzeugen
generate:
	go generate ./...

test:
	go test ./...

build:
	mkdir -p $(DIST)
	tinygo build -no-debug -o $(DIST)/plugin.wasm -target wasip1 -buildmode=c-shared .

# Der Dateiname bestimmt die Plugin-ID in Navidrome – daher immer cantilune.ndp (ohne Version).
package: build
	@if command -v jq >/dev/null 2>&1; then \
		jq --arg v "$(or $(VERSION),0.0.0-dev)" --arg w "$(WEBSITE)" \
			'.version = $$v | if $$w != "" then .website = $$w else . end' \
			manifest.json > $(DIST)/manifest.json; \
	else \
		cp manifest.json $(DIST)/manifest.json; \
	fi
	cd $(DIST) && rm -f $(PLUGIN).ndp && zip -j $(PLUGIN).ndp manifest.json plugin.wasm
	@echo "Paket erstellt: $(DIST)/$(PLUGIN).ndp"

clean:
	rm -rf $(DIST)
