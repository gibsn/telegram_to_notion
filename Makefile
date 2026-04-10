TARGET_BRANCH ?= origin/main
LINT_GOTOOLCHAIN ?= go1.22.4
override GOCACHE := $(CURDIR)/.cache/go-build
override GOPATH := $(CURDIR)/.cache/go
override GOMODCACHE := $(GOPATH)/pkg/mod

all: telegram_to_notion test_request

telegram_to_notion: test
	GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) \
		go build -mod vendor -o ./bin/telegram_to_notion github.com/gibsn/telegram_to_notion/cmd/telegram_to_notion

test_request:
	GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) \
		go build -mod vendor -o ./bin/test_request github.com/gibsn/telegram_to_notion/cmd/test_request

get_chat_id:
	GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) \
		go build -mod vendor -o ./bin/get_chat_id github.com/gibsn/telegram_to_notion/cmd/get_chat_id

bin/golangci-lint:
	@echo "getting golangci-lint for $$(uname -m)/$$(uname -s)"
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.64.8

lint: bin/golangci-lint
	GOTOOLCHAIN=$(LINT_GOTOOLCHAIN) \
		GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) \
		bin/golangci-lint run -v -c ./build/ci/golangci.yml \
		--new-from-rev=$(TARGET_BRANCH)                 \

install: lint test clean telegram_to_notion
	GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) go install ./...

clean:
	rm -rf bin/

test:
	GOCACHE=$(GOCACHE) GOPATH=$(GOPATH) GOMODCACHE=$(GOMODCACHE) go test ./...

.PHONY: all test_request telegram_to_notion get_chat_id lint clean test
