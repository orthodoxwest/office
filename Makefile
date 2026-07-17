.PHONY: help build test test-race parity lint lint-texts vet fmt fmt-check check serve ordo validate audit project-status verify-psalms review-manifest review-status review-provenance review-provenance-queue review-suspects review-plan review-assurance review-sources tex pdf golden clean

YEAR ?= 2026

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o office ./cmd/server

test: ## Run all tests
	go test ./...
	python3 scripts/test_ordo_compare.py
	python3 scripts/test_project_status.py
	python3 scripts/test_source_reconcile.py

test-race: ## Run Go tests with the race detector
	go test -race ./...

parity: ## Verify the 2026-2053 date-sensitive parity snapshot
	go test ./internal/e2e -run TestParityGolden -count=1

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

project-status: build ## Generate clergy-facing proper, assurance, and YEAR ordo status
	python3 scripts/project-status.py --year $(YEAR)

verify-psalms: ## Compare the Coverdale psalter against the official 1662 BCP witness
	go run scripts/verify-psalms.go

review-manifest: build ## Print human-review checklist CSV for current year (START=2026 YEARS=1)
	./office review manifest $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-status: build ## Report human-review coverage vs data/review/signoffs.txt
	./office review status $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-provenance: build ## Report generated corpus provenance coverage
	./office review provenance

review-provenance-queue: build ## Rank atomic text review by rendered dependency fan-out
	./office review provenance-queue $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-suspects: build ## Print only pre-flagged/lint-flagged texts — the findings-sprint list
	./office review provenance-queue -suspect-only $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-plan: build ## Print minimal coverage-oriented review checklist CSV
	./office review plan $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-assurance: build ## Run release assurance coverage gates
	./office review assurance

review-sources: build ## Build disposable source-vs-corpus review packets under output/
	python3 scripts/source-reconcile.py build --resources ../resources --data data --office ./office --output output/source-reconcile

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

golden: ## Regenerate rendered-office and assurance golden files
	go test ./internal/e2e/ -update -count=1

clean: ## Remove build artifacts
	rm -f office
	rm -rf output/
