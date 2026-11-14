(identifier) @variable
(word_definition name: (identifier) @function)
(call_expression function: (identifier) @function)
(parameter name: (identifier) @variable.parameter)
(type) @type
(error_type) @type
"Result" @type
(integer) @number
(boolean) @boolean
(string) @string
(escape_sequence) @string.escape
(comment) @comment

["word" "type" "record" "choice" "match" "case" "use" "let" "var" "if" "elif" "for" "in" "flow" "try" "catch" "return"] @keyword
(else_header) @keyword
["and" "or" "not" "+" "-" "*" "/" "%" "==" "!=" "<" "<=" ">" ">=" "=" "->"] @operator
["(" ")" "[" "]"] @punctuation.bracket
["," ":"] @punctuation.delimiter
"range" "file.lines" @function
