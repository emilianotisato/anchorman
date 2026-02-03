PREFIX ?= ~/.local
BINARY = anchorman

.PHONY: build install uninstall clean run fmt lint test

build:
	go build -o $(BINARY) ./cmd/anchorman

install: build
	mkdir -p $(PREFIX)/bin
	cp $(BINARY) $(PREFIX)/bin/$(BINARY).tmp
	mv -f $(PREFIX)/bin/$(BINARY).tmp $(PREFIX)/bin/$(BINARY)
	@echo "Installed $(BINARY) to $(PREFIX)/bin"
	@echo "Make sure $(PREFIX)/bin is in your PATH"

uninstall:
	rm -f $(PREFIX)/bin/$(BINARY)
	@echo "Uninstalled $(BINARY)"

clean:
	rm -f $(BINARY)

run: build
	./$(BINARY)

# Development helpers
fmt:
	go fmt ./...

lint:
	golangci-lint run

test:
	go test ./...

# Database helpers
db-reset:
	rm -f ~/.anchorman/db/anchorman.sqlite
	@echo "Database reset. Will be recreated on next run."
