#!/bin/sh
set -eu

: "${HOME:?HOME must be set}"
runtime=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
data_home=${XDG_DATA_HOME:-$HOME/.local/share}
for directory in "$HOME" "$data_home"; do
    case "$directory" in
        /*) ;;
        *) echo 'HOME and XDG_DATA_HOME must be absolute paths' >&2; exit 1 ;;
    esac
done
vim="$HOME/.vim/pack/zhex/start/zhex"
nvim="$data_home/nvim/site/pack/zhex/start/zhex"

# Check both destinations before changing either editor's plugin directory.
for destination in "$vim" "$nvim"; do
    if [ -L "$destination" ] && [ "$(readlink "$destination")" = "$runtime" ]; then
        continue
    fi
    if [ -e "$destination" ] || [ -L "$destination" ]; then
        printf 'Refusing to replace existing plugin: %s\n' "$destination" >&2
        exit 1
    fi
done

for destination in "$vim" "$nvim"; do
    if [ ! -L "$destination" ]; then
        mkdir -p "$(dirname -- "$destination")"
        ln -s "$runtime" "$destination"
    fi
    printf 'Installed Zhex syntax: %s\n' "$destination"
done
