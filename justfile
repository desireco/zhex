# List available development commands.
default:
    @just --list

# Run interpreter and website tests.
test: doctests aoc
    go test ./...
    npm --prefix site run build

# Check every example's ## > doctests and run its .test.zhex files with one built binary.
doctests: build
    bin/zhex test examples

# Run the AoC 2025 suite: every aoc/2025 *.test.zhex file with one built binary.
aoc: build
    bin/zhex test aoc/2025

# Run the WASM corpus gate: every examples/ file classified, in-subset byte-identical, out-of-subset fenced.
corpus:
    go test ./internal/wasm/ -run 'TestCorpusGate' -v

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

# Build bin/zhex and copy it to a bin folder, default ~/.local/bin: just install [dir]
install $dir='$HOME/.local/bin': build
    mkdir -p "{{dir}}"
    cp bin/zhex "{{dir}}"

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
