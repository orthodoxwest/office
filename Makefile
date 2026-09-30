.PHONY: help build test test-ux parity lint lint-js lint-texts fmt fmt-check check serve ordo validate audit scaffold-propers project-status verify-psalms review-manifest review-provenance review-provenance-queue review-zero-occurrences review-resolution-inventory review-suspects review-plan review-assurance diurnal-test pages transcribe transcribe-report discover discover-report tex pdf golden rust-check clean sweep-targets mutate mutate-diff test-coverage android android-screenshots ios

.DEFAULT_GOAL := help

YEAR ?= 2026

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

build: ## Build the Rust binary
	cargo build --locked --release -p office-cli
	cp target/release/office office

test: ## Run Rust and Python tests
	cargo test --workspace --locked
	python3 scripts/test_static_contracts.py
	python3 scripts/test_verify_psalms.py
	python3 scripts/test_sweep_targets.py
	python3 scripts/test_ordo_compare.py
	python3 scripts/test_project_status.py
	python3 scripts/test_update_golden_workflow.py
	python3 scripts/test_diurnal_pages.py
	python3 scripts/test_diurnal_transcribe.py
	python3 scripts/test_diurnal_discover.py

# Optional local diagnostic; normal checks do not collect coverage.
test-coverage: ## Inspect local Rust coverage (requires cargo-llvm-cov and llvm-tools-preview)
	mkdir -p output/coverage
	cargo llvm-cov --workspace --locked --lcov --output-path output/coverage/lcov.info

diurnal-test: ## Run page-image, transcription, and discovery unit tests
	python3 scripts/test_diurnal_pages.py
	python3 scripts/test_diurnal_transcribe.py
	python3 scripts/test_diurnal_discover.py

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

discover: build ## Prepare a scan work queue; SLOTS=collect,chapter-lauds narrows the search
	python3 scripts/diurnal-discover.py run --page-key "$(DIURNAL_PAGE_KEY)" $(if $(filter 1,$(APPLY)),--apply,--dry-run) $(if $(FEASTS),--feasts "$(FEASTS)",) $(if $(MONTH),--month "$(MONTH)",) $(if $(LIMIT),--limit "$(LIMIT)",) $(if $(SLOTS),--slots "$(SLOTS)",)

.PHONY: discover-resume
discover-resume: build ## Read the next 3 prepared tasks; APPLY=1 enables existing application checks
	@test -n "$(RUN)" || (echo "RUN is required" >&2; exit 2)
	python3 scripts/diurnal-discover.py resume "$(RUN)" $(if $(LIMIT),--limit "$(LIMIT)",) $(if $(FEASTS),--feasts "$(FEASTS)",) $(if $(filter 1,$(RETRY)),--retry,) $(if $(filter 1,$(APPLY)),--apply,)

discover-report: ## Print discovery PR markdown for RUN=<run-id-or-directory>
	@test -n "$(RUN)" || (echo "RUN is required" >&2; exit 2)
	python3 scripts/diurnal-discover.py report "$(RUN)"

test-ux: build ## Run Playwright UX regression tests against Rust
	npm --prefix .web-tools run test:ux

lint: ## Run Clippy
	cargo clippy --workspace --all-targets --locked -- -D warnings

lint-js: ## Run ESLint on browser and service-worker JavaScript
	npm --prefix .web-tools run lint

fmt: ## Reformat Rust source files
	cargo fmt

fmt-check: ## Check Rust formatting
	cargo fmt --check

lint-texts: build ## Lint the text corpus (mechanical findings fail; advisory printed)
	./office lint

check: fmt-check lint lint-js test validate lint-texts ## Run all formatting, static analysis, tests, and data checks

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
	python3 scripts/verify-psalms.py

review-manifest: build ## Inventory distinct rendered compositions for current year (START=2026 YEARS=1)
	./office review manifest $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-provenance: build ## Report generated corpus provenance coverage (+ usage-weighted %; START/YEARS scope the sweep)
	./office review provenance $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-provenance-queue: build ## Rank atomic text review by rendered dependency fan-out
	./office review provenance-queue $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-zero-occurrences: build ## List unrendered atomic texts with classification heuristics
	./office review zero-occurrences $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-resolution-inventory: build ## Inventory proper-resolution paths (default 28y; START/YEARS override)
	./office review resolution-inventory -json $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-suspects: build ## Print only pre-flagged/lint-flagged texts — the findings-sprint list
	./office review provenance-queue -suspect-only $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-plan: build ## Sample observed engine behavior (default 28y; START/YEARS override)
	./office review plan $(if $(START),-start $(START),) $(if $(YEARS),-years $(YEARS),)

review-assurance: build ## Check text-provenance floor and print summary
	./office review assurance

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

MUTATE_PKG ?= calendar
MUTATE_DIFF_BASE ?= master

mutate: ## Inspect Rust assertion gaps (requires cargo-mutants)
	cargo mutants --package $(MUTATE_PKG) --output output/mutation --jobs 1 --timeout 60 --gitignore true

mutate-diff: ## Inspect mutations in changed Rust lines
	mkdir -p output/mutation
	git diff $(MUTATE_DIFF_BASE) -- '*.rs' > output/mutation/changes.diff
	cargo mutants --package $(MUTATE_PKG) --in-diff output/mutation/changes.diff --output output/mutation --jobs 1 --timeout 60 --gitignore true

rust-check: ## Rust workspace: fmt, clippy, and tests
	cargo fmt --check
	cargo clippy --workspace --all-targets --locked -- -D warnings
	cargo test --workspace --locked

parity: build ## Check every snapshot, including the 2026–2053 digest
	python3 scripts/golden.py --check

golden: build ## Regenerate rendered-office and assurance golden files
	python3 scripts/golden.py

android: ## Build the sideloadable Android preview APK (needs Android SDK/NDK and cargo-ndk; see apps/android/README.md)
	cd apps/android && ./gradlew assemblePreview
	@echo "APK: apps/android/app/build/outputs/apk/preview/app-preview.apk"

android-screenshots: ## Render Android screens from the Rust core into apps/android/app/build/screenshots/
	cd apps/android && ./gradlew testDebugUnitTest

ios: ## Build the iOS core and generate the Xcode project (macOS with Xcode and XcodeGen; see apps/ios/README.md)
	apps/ios/build-core.sh
	cd apps/ios && xcodegen generate
	@echo "Open apps/ios/Office.xcodeproj in Xcode"

clean: ## Remove build artifacts
	cargo clean
	rm -f office
	rm -rf output/

SWEEP_HOURS ?= 72

sweep-targets: ## Delete target/ in checkouts idle for SWEEP_HOURS (DRY_RUN=1 to preview)
	python3 scripts/sweep-targets.py --hours $(SWEEP_HOURS) $(if $(DRY_RUN),--dry-run)
