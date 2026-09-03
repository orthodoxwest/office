.PHONY: help install-hooks build test test-race test-ux parity lint lint-js lint-texts vet fmt fmt-check check serve ordo validate audit scaffold-propers project-status verify-psalms review-manifest review-status review-provenance review-provenance-queue review-zero-occurrences review-resolution-inventory review-suspects review-plan review-assurance review-sources review-agent-plan review-agent-run review-agent-status review-agent-collect review-agent-adjudicate review-agent-test diurnal-doctor diurnal-test pages transcribe transcribe-report tex pdf golden clean install-gremlins mutate mutate-diff mutate-ratchet

YEAR ?= 2026

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

install-hooks: ## Install the repository's versioned Git hooks
	git config core.hooksPath .githooks
	@echo "Git hooks installed. Pre-push will run: make check"

build: ## Build the binary
	go build -o office ./cmd/server

test: ## Run all tests
	go test ./...
	python3 scripts/test_mutation_threshold.py
	python3 scripts/test_ordo_compare.py
	python3 scripts/test_project_status.py
	python3 scripts/test_source_reconcile.py
	python3 scripts/test_diurnal_intake.py
	python3 scripts/test_diurnal_agent.py
	python3 scripts/test_diurnal_pages.py
	python3 scripts/test_diurnal_transcribe.py

diurnal-doctor: ## Check PDF/OCR tools needed for diurnal intake
	python3 scripts/diurnal-intake.py doctor

diurnal-test: ## Run PDF intake unit tests
	python3 scripts/test_diurnal_intake.py
	python3 scripts/test_diurnal_pages.py
	python3 scripts/test_diurnal_transcribe.py

BOOKS_DIR ?= ../resources/books
DIURNAL_PDF ?= $(BOOKS_DIR)/Monastic Diurnal.pdf
DIURNAL_PAGE_KEY ?= monastic-diurnal
DPI ?= 150

pages: ## Render and index the Monastic Diurnal and supplement PDFs
	python3 scripts/diurnal-pages.py render "$(DIURNAL_PDF)" --key "$(DIURNAL_PAGE_KEY)" --dpi "$(DPI)"
	@if test -d "$(BOOKS_DIR)/supplements"; then \
		find "$(BOOKS_DIR)/supplements" -maxdepth 1 -type f -name '*.pdf' -exec sh -c 'for pdf do base=$$(basename "$$pdf" .pdf); key=$$(printf "%s" "$$base" | tr "[:upper:]" "[:lower:]" | sed "s/[^a-z0-9._-]/-/g; s/--*/-/g; s/^-//; s/-$$//"); python3 scripts/diurnal-pages.py render "$$pdf" --key "supplement-$$key" --dpi "$(DPI)" || exit; done' sh {} +; \
	fi

transcribe: build ## Prepare prompts by default; APPLY=1 invokes readers and applies gated results
	python3 scripts/diurnal-transcribe.py run --page-key "$(DIURNAL_PAGE_KEY)" $(if $(filter 1,$(APPLY)),--apply,--dry-run) $(if $(KEYS),--keys "$(KEYS)",)

transcribe-report: ## Print markdown for RUN=<run-id-or-directory>
	@test -n "$(RUN)" || (echo "RUN is required" >&2; exit 2)
	python3 scripts/diurnal-transcribe.py report "$(RUN)"

test-race: ## Run Go tests with the race detector
	go test -race ./...

test-ux: ## Run Playwright UX regression tests
	npm --prefix .web-tools run test:ux

parity: ## Verify the 2026-2053 date-sensitive parity snapshot
	go test ./internal/e2e -run TestParityGolden -count=1

lint: ## Run staticcheck linter
	staticcheck ./...

lint-js: ## Run ESLint on browser and service-worker JavaScript
	npm --prefix .web-tools run lint

vet: ## Run go vet
	go vet ./...

fmt: ## Reformat source files with gofmt
	gofmt -w .

fmt-check: ## Check formatting without modifying files
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

lint-texts: build ## Lint the text corpus (mechanical findings fail; advisory printed)
	./office lint

check: fmt-check vet lint lint-js test validate lint-texts ## Run all formatting, static analysis, tests, and data checks

serve: build ## Start the web server
	./office serve

ordo: build ## Print text ordo for YEAR (default 2026)
	./office ordo $(YEAR)

validate: build ## Validate data files
	./office validate

audit: build ## Report placeholder texts and missing feast propers
	./office audit

scaffold-propers: build ## Ensure proper text files exist with commented key catalogs (never overwrites live sections)
	./office scaffold propers

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

review-zero-occurrences: build ## List unrendered atomic texts with classification heuristics
	./office review zero-occurrences $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-resolution-inventory: build ## Inventory proper-resolution paths (default 28y; START/YEARS override)
	./office review resolution-inventory -json $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-suspects: build ## Print only pre-flagged/lint-flagged texts — the findings-sprint list
	./office review provenance-queue -suspect-only $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-plan: build ## Print residual structural checklist CSV (default 28y fan-out; START/YEARS override)
	./office review plan $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-assurance: build ## Run release assurance coverage gates
	./office review assurance

review-sources: build ## Build disposable source-vs-corpus review packets under output/
	python3 scripts/source-reconcile.py build --resources ../resources --data data --office ./office --output output/source-reconcile

DISCOVERY ?= output/source-reconcile/proper-discovery.json

review-agent-plan: ## Plan read-only agent jobs from DISCOVERY
	python3 scripts/diurnal-agent.py plan $(DISCOVERY)

review-agent-run: ## Dry-run the current agent plan (add --execute explicitly at the CLI)
	python3 scripts/diurnal-agent.py run

review-agent-status: ## Report the current agent run state
	python3 scripts/diurnal-agent.py status

review-agent-collect: ## Collect non-stale structured agent proposals
	python3 scripts/diurnal-agent.py collect

review-agent-adjudicate: ## Plan Claude jobs for conflicting replicas (must be enabled in policy)
	python3 scripts/diurnal-agent.py adjudicate

review-agent-test: ## Run provider-runner tests without invoking providers
	python3 scripts/test_diurnal_agent.py

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

GREMLINS_VERSION = v0.6.0
GREMLINS_BIN = $(shell go env GOBIN)
ifeq ($(GREMLINS_BIN),)
GREMLINS_BIN = $(shell go env GOPATH)/bin
endif
GREMLINS = $(GREMLINS_BIN)/gremlins
MUTATE_PKGS ?= ./internal/models/ ./internal/calendar/ ./internal/office/ ./internal/texts/
MUTATE_DIFF_BASE ?= master
MUTATE_RATCHET ?=
MUTATE_RATCHETS ?= \
	models:./internal/models/:100:100 \
	calendar:./internal/calendar/:86:92 \
	office:./internal/office/:86:92

# In --diff mode gremlins measures its baseline over the whole suite (~30s)
# rather than one package (~1s), so the coefficient pinned in .gremlins.yaml
# would yield a ~15-minute per-mutant timeout. Override it here; the large
# baseline already makes a small coefficient generous.
MUTATE_DIFF_COEFFICIENT ?= 5

# `gremlins version` reports "dev" for go-install builds, but the module version
# is recorded in the binary and readable with `go version -m`. Check that rather
# than mere presence: CI restores ~/go/bin from a prefix-matched cache, so a
# presence-only check would silently keep running a stale version after a bump.
install-gremlins: ## Install the pinned mutation-testing tool if missing or stale
	@go version -m $(GREMLINS) 2>/dev/null | grep -q 'gremlins[[:space:]]*$(GREMLINS_VERSION)' || \
		go install github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)

# Note: thresholds are 0, so gremlins exits 0 regardless of efficacy. The
# `|| exit 1` catches hard failures (compile errors, crashes), not bad scores.
mutate: install-gremlins ## Mutation-test whole packages without enforcing the CI ratchets
	@for pkg in $(MUTATE_PKGS); do \
		echo "==> $$pkg"; \
		$(GREMLINS) unleash $$pkg || exit 1; \
	done

# Must run from the module root with no package path: passing a path alongside
# --diff makes gremlins skip every mutant, including the changed ones, and
# still exit 0.
mutate-diff: install-gremlins ## Mutation-test only lines changed vs MUTATE_DIFF_BASE (default master)
	$(GREMLINS) unleash --diff $(MUTATE_DIFF_BASE) \
		--timeout-coefficient $(MUTATE_DIFF_COEFFICIENT)

mutate-ratchet: install-gremlins ## Enforce core mutation floors (MUTATE_RATCHET=models selects one package)
	@selected='$(MUTATE_RATCHET)'; \
	found=0; \
	report=''; \
	trap 'test -z "$$report" || rm -f "$$report"' EXIT HUP INT TERM; \
	for spec in $(MUTATE_RATCHETS); do \
		package=$${spec%%:*}; \
		values=$${spec#*:}; \
		path=$${values%%:*}; \
		values=$${values#*:}; \
		efficacy=$${values%%:*}; \
		mcover=$${values#*:}; \
		if test -n "$$selected" && test "$$selected" != "$$package"; then \
			continue; \
		fi; \
		found=1; \
		echo "==> mutation ratchet: $$package"; \
		report=$$(mktemp); \
		$(GREMLINS) unleash "$$path" --output "$$report" || exit 1; \
		python3 scripts/check_mutation_threshold.py "$$report" \
			--min-efficacy "$$efficacy" \
			--min-mutant-coverage "$$mcover" || exit 1; \
		rm -f "$$report"; \
		report=''; \
	done; \
	if test "$$found" -eq 0; then \
		echo "Unknown mutation ratchet: $$selected" >&2; \
		exit 2; \
	fi

golden: ## Regenerate rendered-office and assurance golden files
	go test ./internal/e2e/ -update -count=1

clean: ## Remove build artifacts
	rm -f office
	rm -rf output/
