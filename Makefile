APP=bilichat
BUILD_DIR=./build

build:
	@go mod tidy
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP)-linux-amd64
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP)-linux-arm64
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP)-windows-amd64.exe
	@CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP)-darwin-amd64
	@CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o $(BUILD_DIR)/$(APP)-darwin-arm64

clean:
	@rm -r $(BUILD_DIR)

run:
	@go run main.go

debug:
	@BILICHAT_DEBUG=1 go run main.go
