set nomore
let s:plugin = fnamemodify(expand('<sfile>:p'), ':h:h')
execute 'set runtimepath^=' . fnameescape(s:plugin)
filetype plugin on
syntax enable
execute 'edit ' . fnameescape(s:plugin . '/../testdata/highlighting.zhex')

call assert_equal('zhex', &filetype)
call assert_equal('zhex', b:current_syntax)
call assert_equal(4, &shiftwidth)
call assert_true(&expandtab)
call assert_equal('# %s', &commentstring)

function! s:Check(needle, offset, expected) abort
  for lnum in range(1, line('$'))
    let start = stridx(getline(lnum), a:needle)
    if start >= 0
      call assert_equal(a:expected, synIDattr(synID(lnum, start + a:offset + 1, 1), 'name'), a:needle)
      return
    endif
  endfor
  call assert_report('Missing fixture text: ' . a:needle)
endfunction

call s:Check('# word', 2, 'zhexComment')
call s:Check('word is-palindrome', 0, 'zhexKeyword')
call s:Check('word is-palindrome', 5, 'zhexDefinition')
call s:Check('value: String', 0, 'zhexParameter')
call s:Check('value: String', 7, 'zhexType')
call s:Check('let if-ready', 4, 'zhexIdentifier')
call s:Check('= true', 2, 'zhexBoolean')
call s:Check('= -12', 3, 'zhexNumber')
call s:Check('text.reverse(value)', 0, 'zhexBuiltin')
call s:Check('count - 1', 6, 'zhexOperator')
call s:Check('"if #', 1, 'zhexString')
call s:Check('\n"', 0, 'zhexEscape')

call append('$', ['let broken = "unfinished', 'print(1)'])
call assert_equal('zhexBuiltin', synIDattr(synID(line('$'), 1, 1), 'name'))

if len(v:errors)
  for message in v:errors
    echom message
  endfor
  cquit
endif
quitall!
