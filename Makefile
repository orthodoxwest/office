.PHONY: help build test lint lint-texts vet fmt fmt-check check serve ordo validate audit review-manifest review-status tex pdf golden clean

YEAR ?= 2026

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o office ./cmd/server

test: ## Run all tests
	go test ./...

lint: ## Run staticcheck linter
	staticcheck ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Reformat source files with gofmt
	gofmt -w .

fmt-check: ## Check formatting without modifying files
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

lint-texts: build ## Lint the text corpus (mechanical findings fail; advisory printed)
	./office lint

check: fmt-check vet lint test validate lint-texts ## Run fmt + vet + lint + test + validate + lint-texts

serve: build ## Start the web server
	./office serve

ordo: build ## Print text ordo for YEAR (default 2026)
	./office ordo $(YEAR)

validate: build ## Validate data files
	./office validate

audit: build ## Report placeholder texts and missing feast propers
	./office audit

review-manifest: build ## Print human-review checklist CSV for current year (START=2026 YEARS=1)
	./office review manifest $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-status: build ## Report human-review coverage vs data/review/signoffs.txt
	./office review status $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

DATE ?= $(shell date +%Y-%m-%d)
CHANT ?=
CHANT_FLAG = $(if $(CHANT),--chant,)

tex: build ## Generate .tex for HOUR [DATE] [CHANT=1] (e.g., make tex HOUR=lauds DATE=2026-03-11)
	./office tex $(CHANT_FLAG) $(HOUR) $(DATE)

pdf: build ## Generate PDF booklet for HOUR [DATE] [CHANT=1] (e.g., make pdf HOUR=compline CHANT=1)
	mkdir -p output
	./office tex $(CHANT_FLAG) $(HOUR) $(DATE) > output/$(HOUR)-$(DATE).tex
	lualatex --shell-escape --interaction=nonstopmode --output-directory=output output/$(HOUR)-$(DATE).tex
	@echo "PDF: output/$(HOUR)-$(DATE).pdf"

golden: ## Regenerate golden test files
	go test ./internal/e2e/ -update -count=1

clean: ## Remove build artifacts
	rm -f office
	rm -rf output/
