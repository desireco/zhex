#!/bin/sh
set -eu

runtime=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT HUP INT TERM

# Exercise paths with spaces without touching the user's editor directories.
source="$temporary/source checkout/vim"
mkdir -p "$source"
cp "$runtime/install.sh" "$source/install.sh"
cp -R "$runtime/ftdetect" "$runtime/ftplugin" "$runtime/syntax" "$source/"
export HOME="$temporary/user home"
export XDG_DATA_HOME="$temporary/editor data"
vim="$HOME/.vim/pack/zhex/start/zhex"
nvim="$XDG_DATA_HOME/nvim/site/pack/zhex/start/zhex"

sh "$source/install.sh"
test "$(readlink "$vim")" = "$source"
test "$(readlink "$nvim")" = "$source"
sh "$source/install.sh"

# A conflict in either destination must preserve it and leave the other alone.
rm "$vim" "$nvim"
mkdir "$nvim"
printf 'keep\n' > "$nvim/existing"
if sh "$source/install.sh" > "$temporary/output" 2>&1; then
    echo 'installer accepted an existing directory' >&2
    exit 1
fi
test ! -e "$vim"
test ! -L "$vim"
test "$(cat "$nvim/existing")" = keep
case "$(cat "$temporary/output")" in
    *'Refusing to replace'*) ;;
    *) echo 'missing conflict diagnostic' >&2; exit 1 ;;
esac

rm -r "$nvim"
ln -s "$temporary/other plugin" "$vim"
if sh "$source/install.sh" > "$temporary/output" 2>&1; then
    echo 'installer accepted a different symlink' >&2
    exit 1
fi
test "$(readlink "$vim")" = "$temporary/other plugin"
test ! -e "$nvim"
test ! -L "$nvim"
rm "$vim"

unset XDG_DATA_HOME
sh "$source/install.sh"
test "$(readlink "$HOME/.local/share/nvim/site/pack/zhex/start/zhex")" = "$source"

cat > "$temporary/init.vim" <<'EOF'
filetype plugin on
syntax enable
EOF
cat > "$temporary/check.vim" <<'EOF'
execute 'edit ' . fnameescape($ZHEX_FIXTURE)
call assert_equal('zhex', &filetype)
call assert_equal('zhex', get(b:, 'current_syntax', ''))
call assert_equal('# %s', &commentstring)
if !empty(v:errors)
    echo join(v:errors, "\n")
    cquit
endif
qa!
EOF
export ZHEX_FIXTURE="$runtime/../testdata/highlighting.zhex"
if command -v vim >/dev/null 2>&1; then
    vim -Nu "$temporary/init.vim" -n -es -S "$temporary/check.vim"
fi
if command -v nvim >/dev/null 2>&1; then
    NVIM_APPNAME=nvim nvim --headless -u "$temporary/init.vim" -n -S "$temporary/check.vim"
fi
echo 'Vim and Neovim installation checks passed'
