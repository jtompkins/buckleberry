# Using .PHONY declares these targets as not being actual files.
# This is a good practice for targets that are commands.
.PHONY: all run build generate test fmt watch build

# The default target executed when you just run `make`
all: build

# Run the application
run: generate
	go run main.go

build: generate
	go build -o out/buckleberry main.go

# Run the tests
test:
	go test ./...

# Format the Go source code
fmt:
	go fmt ./...

db.reset:
	rm *.db

generate:
	go tool templ generate

watch:
	go tool templ generate --watch --proxy=http://localhost:8080 --cmd="go run ."
