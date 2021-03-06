GOCMD=go
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
EXPORT_RESULT?=false

GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))
BINARY_NAME := $(current_dir)

.PHONY: help

## Build:
build: ## Build your project and put the output binary in out/bin/
	@echo "$(GREEN)Building... $(BINARY_NAME)$(RESET)"
	mkdir -p out/bin
	$(GOCMD) build -o out/bin/$(BINARY_NAME) .

build-all: \
	build-linux-arm64 \
	build-linux-amd64 \
	build-darwin-arm64 \
	build-darwin-amd64 \
	build-windows-amd64 \
	build-windows-arm64 ## Build your project for all OS' and ARCH and put the output binary in out/bin/


build-linux-arm64: ## Build your project for linux arm64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=linux GOARCH=arm64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-linux-arm64 .

build-linux-amd64: ## Build your project for linux amd64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=linux GOARCH=arm64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-linux-amd64 .

build-darwin-arm64: ## Build your project for darwin arm64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=darwin GOARCH=arm64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-darwin-arm64 .

build-darwin-amd64: ## Build your project for darwin amd64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=darwin GOARCH=amd64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-darwin-amd64 .

build-windows-amd64: ## Build your project for windows amd64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=windows GOARCH=amd64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-windows-amd64 .

build-windows-arm64: ## Build your project for windows arm64 and put the output binary in out/bin/
	mkdir -p out/bin
	GOOS=windows GOARCH=arm64 $(GOCMD) build -o out/bin/$(BINARY_NAME)-windows-arm64 .

clean: ## Remove build related file
	rm -fr ./bin
	rm -fr ./out
	rm -f ./junit-report.xml checkstyle-report.xml ./coverage.xml ./profile.cov yamllint-checkstyle.xml

check: lint test ## Run all checks of the project running lint and tests

## Test:
test: ## Run the tests of the project
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/jstemmer/go-junit-report
	$(eval OUTPUT_OPTIONS = | tee /dev/tty | go-junit-report -set-exit-code > junit-report.xml)
endif
	$(GOTEST) -v -race ./... $(OUTPUT_OPTIONS)

coverage: ## Run the tests of the project and export the coverage
	$(GOTEST) -cover -covermode=count -coverprofile=profile.cov ./...
	$(GOCMD) tool cover -func profile.cov
ifeq ($(EXPORT_RESULT), true)
	GO111MODULE=off go get -u github.com/AlekSi/gocov-xml
	GO111MODULE=off go get -u github.com/axw/gocov/gocov
	gocov convert profile.cov | gocov-xml > coverage.xml
endif

## Lint:
lint: lint-go ## Run all available linters

lint-go: ## Use golintci-lint on your project
	$(eval OUTPUT_OPTIONS = $(shell [ "${EXPORT_RESULT}" == "true" ] && echo "--out-format checkstyle ./... | tee /dev/tty > checkstyle-report.xml" || echo "" ))
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:latest-alpine golangci-lint run \
		--deadline=65s $(OUTPUT_OPTIONS)

## Help:
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
