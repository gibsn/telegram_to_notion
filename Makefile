all: telegram_to_notion test_request

telegram_to_notion:
	go build -mod vendor -o ./bin/telegram_to_notion github.com/gibsn/telegram_to_notion/cmd/telegram_to_notion

test_request:
	go build -mod vendor -o ./bin/test_request github.com/gibsn/telegram_to_notion/cmd/test_request

bin/golangci-lint:
	@echo "getting golangci-lint for $$(uname -m)/$$(uname -s)"
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.64.8

lint: bin/golangci-lint
	bin/golangci-lint run -v -c ./build/ci/golangci.yml \
		--new-from-rev=$(TARGET_BRANCH)                 \

install: lint clean telegram_to_notion
	go install ./...

clean:
	rm -rf bin/

test:
	go test ./...

.PHONY: all test_request telegram_to_notion lint clean test

