BINARY_NAME := twscrape

INSTALL_DIR := $(HOME)/bin

TWITTER_HANDLE := X

all: $(BINARY_NAME)

$(BINARY_NAME): $(BINARY_NAME).go
	go build -o $(BINARY_NAME) $(BINARY_NAME).go

run: $(BINARY_NAME)
	./$(BINARY_NAME) $(TWITTER_HANDLE)

install: $(BINARY_NAME).go
	install -m 0755 $(BINARY_NAME) $(INSTALL_DIR)

clean:
	go clean

