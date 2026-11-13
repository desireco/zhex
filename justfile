# List available development commands.
default:
    @just --list

# Run interpreter and website tests.
test:
    go test ./...
    npm --prefix site run build

# Run tests with Go's race detector.
test-race:
    go test -race ./...

# Check Go source with vet.
check:
    go vet ./...

# Benchmark the interpreter with allocation counts.
bench:
    go test -bench=. -benchmem -benchtime=2s ./internal/lang/

# Build the interpreter in bin/zhex.
build:
    go build -o bin/zhex ./cmd/zhex

# Run an example by name, such as: just run fibonacci.
run $example='hello':
    go run ./cmd/zhex run "examples/$example.zhex"

# Build the website from the working examples and roadmap.
site:
    npm --prefix site run build

# Prepare the local Zed language extension.
zed-prepare:
    go run ./tools/prepare-zed

# Install syntax support for Vim and Neovim.
vim-install:
    sh editors/vim/install.sh

# Build and preview the website at http://127.0.0.1:8000/zhex/.
serve $port='8000': site
    npm --prefix site run preview -- --host 127.0.0.1 --port "$port"
