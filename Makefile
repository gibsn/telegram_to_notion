telegram_to_notion:
	go build -mod vendor -o ./bin/telegram_to_notion github.com/gibsn/telegram_to_notion

bin/golangci-lint:
	@echo "getting golangci-lint for $$(uname -m)/$$(uname -s)"
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s v1.64.8

lint: bin/golangci-lint
	bin/golangci-lint run -v -c ./build/ci/golangci.yml \
		--new-from-rev=$(TARGET_BRANCH)                 \

clean:
	rm -rf bin/

.PHONY: telegram_to_notion lint clean

