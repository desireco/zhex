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
syntax keyword zhexKeyword type record choice match case use let var if elif else for while in flow with try catch return
syntax keyword zhexType Int Bool String Unit Buffer File Error Result List Array Map Option Date Time Duration Instant TimeZone ZonedDateTime InputError ParseError FileError BufferError
syntax keyword zhexBoolean true false
syntax keyword zhexOperator and or not
syntax keyword zhexBuiltin array.length array.new array.select array.rank array.resize array.shape buffer.add-line buffer.line buffer.line-count buffer.new buffer.set-line buffer.to-text console.arg console.args-count console.exit console.fail date.add-days date.new date.parse date.to-string duration.new duration.parse duration.to-string env.get env.load err error.cause error.code error.column error.detail error.file error.has-cause error.line error.message error.new error.operation error.report error.set-reporter error.wrap file.base file.close file.delete file.delete-folder file.directory-of file.exists file.extension file.free-space file.join file.list file.make-folder file.move file.open file.read file.read-line file.size file.stream file.write file.write-bytes instant.add-duration instant.now instant.parse instant.to-string int.mod-floor int.to-string json.decode json.encode json.parse list.all list.any list.append list.contains list.filter list.find list.select list.fold list.length list.map list.new list.sort list.sort-by list.insert map.contains map.into map.keys map.length map.put map.remove map.values ok print range read-line text.contains text.word-count text.ends-with text.from-code-points text.index-of text.join text.length text.line-count text.lower text.replace text.repeat text.reverse text.slice text.split text.starts-with text.to-code-points text.to-int text.trim text.upper text.insert text.pad-left text.pad-right text.remove text.wrap time.new time.parse time.to-string timezone.load timezone.name xml.decode zoned.add-days zoned.add-duration zoned.in-zone zoned.instant zoned.new zoned.to-string
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
