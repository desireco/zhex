if exists('b:current_syntax')
  finish
endif

syntax case match
syntax iskeyword @,48-57,_,45,46

syntax match zhexIdentifier /[A-Za-z_][A-Za-z0-9_]*\%([-\.][A-Za-z_][A-Za-z0-9_]*\)*/
syntax match zhexFunction /[A-Za-z_][A-Za-z0-9_]*\%([-\.][A-Za-z_][A-Za-z0-9_]*\)*\ze\s*(/
syntax match zhexDefinition /[A-Za-z_][A-Za-z0-9_]*\%([-\.][A-Za-z_][A-Za-z0-9_]*\)*/ contained
syntax match zhexParameter /[A-Za-z_][A-Za-z0-9_]*\%([-\.][A-Za-z_][A-Za-z0-9_]*\)*\ze\s*:/
syntax keyword zhexKeyword word nextgroup=zhexDefinition skipwhite
syntax keyword zhexKeyword type record choice match case use let var if elif else for in flow try catch return
syntax keyword zhexType Int Bool String Unit Buffer File Error Result List Array Map Option InputError ParseError FileError BufferError
syntax keyword zhexBoolean true false
syntax keyword zhexOperator and or not
syntax keyword zhexBuiltin print range int.to-string text.reverse text.to-int read-line ok err error.new error.wrap error.has-cause error.cause error.code error.message error.operation error.file error.line error.column error.detail error.report error.set-reporter console.exit console.fail list.new list.length list.append array.new array.resize array.shape array.rank array.length map.put map.into map.length map.contains map.remove
syntax match zhexNumber /\%([A-Za-z0-9_.]\)\@<!\d\+\%([A-Za-z0-9_.]\)\@!/
syntax match zhexOperator /->\|==\|!=\|<=\|>=\|[-+*\/%<>=]/
syntax match zhexDelimiter /[()\[\]{},:]/
syntax region zhexString start=/"/ skip=/\\./ end=/"/ oneline contains=zhexEscape
syntax match zhexEscape /\\\%([abfnrtv\\"]\|[0-7]\{3}\|x[0-9A-Fa-f]\{2}\|u[0-9A-Fa-f]\{4}\|U[0-9A-Fa-f]\{8}\)/ contained
syntax match zhexComment /#.*/ contains=@Spell

highlight default link zhexIdentifier Identifier
highlight default link zhexFunction Function
highlight default link zhexDefinition Function
highlight default link zhexParameter Identifier
highlight default link zhexKeyword Keyword
highlight default link zhexType Type
highlight default link zhexBoolean Boolean
highlight default link zhexBuiltin Function
highlight default link zhexNumber Number
highlight default link zhexOperator Operator
highlight default link zhexDelimiter Delimiter
highlight default link zhexString String
highlight default link zhexEscape SpecialChar
highlight default link zhexComment Comment

let b:current_syntax = 'zhex'
