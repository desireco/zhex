(identifier) @variable
(word_definition name: (identifier) @function)
(call_expression function: (identifier) @function)
(parameter name: (identifier) @variable.parameter)
(type) @type
(integer) @number
(boolean) @boolean
(string) @string
(escape_sequence) @string.escape
(comment) @comment

["word" "let" "var" "if" "elif" "for" "in" "flow" "try"] @keyword
(else_header) @keyword
["and" "or" "not" "+" "-" "*" "/" "%" "==" "!=" "<" "<=" ">" ">=" "=" "->"] @operator
["(" ")"] @punctuation.bracket
["," ":"] @punctuation.delimiter
"range" @function
