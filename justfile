default:
    @just --list

# Build the klens binary
build:
    go build -o klens .

# Install klens to $GOPATH/bin
install:
    go install .

# Run klens (pass args with: just run --namespace production)
run *args:
    go run . {{args}}

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test ./... -v

# Run tests with race detector
test-race:
    go test -race ./...

# Run golangci-lint
lint:
    golangci-lint run ./...

# Run go vet
vet:
    go vet ./...

# Run all checks (test + vet + lint)
check: test vet lint

# Tidy go.mod and go.sum
tidy:
    go mod tidy

# Remove built binary
clean:
    rm -f klens

# Dry-run the GoReleaser pipeline (no publish)
release-dry:
    goreleaser release --snapshot --clean

# Tag and push a release (usage: just release v0.2.0)
release tag:
    git tag {{tag}}
    git push origin {{tag}}
