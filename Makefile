.PHONY: dev
dev:
	while true; do \
		fd . | entr -cd make build; \
	done

.PHONY: build
build:
	go build
