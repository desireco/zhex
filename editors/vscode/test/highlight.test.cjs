const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const textmate = require('vscode-textmate');
const oniguruma = require('vscode-oniguruma');

const root = path.resolve(__dirname, '..');
const grammarPath = path.join(root, 'syntaxes/zhex.tmLanguage.json');
const ready = (async () => {
  const wasm = fs.readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'));
  await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));
  const registry = new textmate.Registry({
    onigLib: Promise.resolve({ createOnigScanner: patterns => new oniguruma.OnigScanner(patterns), createOnigString: value => new oniguruma.OnigString(value) }),
    loadGrammar: async scope => scope === 'source.zhex' ? textmate.parseRawGrammar(fs.readFileSync(grammarPath, 'utf8'), grammarPath) : null,
  });
  return registry.loadGrammar('source.zhex');
})();

test('Zhex TextMate scopes match real syntax', async () => {
  const grammar = await ready;
  const lines = fs.readFileSync(path.join(root, '../testdata/highlighting.zhex'), 'utf8').split('\n');
  let stack = textmate.INITIAL;
  const tokenized = lines.map(line => { const result = grammar.tokenizeLine(line, stack); stack = result.ruleStack; return result.tokens; });
  function expectScope(needle, offset, scope) {
    const row = lines.findIndex(line => line.includes(needle));
    assert.notEqual(row, -1, needle);
    const column = lines[row].indexOf(needle) + offset;
    const token = tokenized[row].find(token => token.startIndex <= column && token.endIndex > column);
    assert.ok(token?.scopes.includes(scope), `${needle}: expected ${scope}, got ${token?.scopes}`);
  }
  expectScope('# word', 2, 'comment.line.number-sign.zhex');
  expectScope('= choice', 2, 'keyword.control.zhex');
  expectScope('match Status.Ready', 0, 'keyword.control.zhex');
  expectScope('case Status.Ready', 0, 'keyword.control.zhex');
  expectScope('type Customer', 0, 'keyword.control.zhex');
  expectScope('List<Int>', 0, 'support.type.builtin.zhex');
  expectScope('Array<Int>', 0, 'support.type.builtin.zhex');
  expectScope('array.new(', 0, 'support.function.builtin.zhex');
  expectScope('use math', 0, 'keyword.control.zhex');
  expectScope('catch problem', 0, 'keyword.control.zhex');
  expectScope('return err', 0, 'keyword.control.zhex');
  expectScope('Result<Int', 0, 'support.type.builtin.zhex');
  expectScope('ParseError>', 0, 'support.type.builtin.zhex');
  expectScope('error.wrap(', 0, 'support.function.builtin.zhex');
  expectScope('console.exit(', 0, 'support.function.builtin.zhex');
  expectScope('console.fail(', 0, 'support.function.builtin.zhex');
  expectScope('word is-palindrome', 0, 'storage.type.function.zhex');
  expectScope('word is-palindrome', 5, 'entity.name.function.zhex');
  expectScope('value: String', 0, 'variable.parameter.zhex');
  expectScope('value: String', 7, 'support.type.builtin.zhex');
  expectScope('let if-ready', 4, 'variable.other.zhex');
  expectScope('= true', 2, 'constant.language.boolean.zhex');
  expectScope('= -12', 3, 'constant.numeric.integer.zhex');
  expectScope('text.reverse(value)', 0, 'support.function.builtin.zhex');
  expectScope('count - 1', 6, 'keyword.operator.zhex');
  expectScope('"if #', 1, 'string.quoted.double.zhex');
  expectScope('\\n"', 0, 'constant.character.escape.zhex');
});

test('unterminated strings stop at the end of their line', async () => {
  const grammar = await ready;
  const first = grammar.tokenizeLine('let value = "unfinished', textmate.INITIAL);
  const second = grammar.tokenizeLine('print(1)', first.ruleStack);
  assert.ok(second.tokens[0].scopes.includes('support.function.builtin.zhex'));
  assert.ok(!second.tokens[0].scopes.includes('string.quoted.double.zhex'));
});

test('all shipped examples tokenize and language configuration uses current syntax', async () => {
  const grammar = await ready;
  const directory = path.join(root, '../../examples');
  for (const filename of fs.readdirSync(directory, { recursive: true }).filter(name => name.endsWith('.zhex'))) {
    let stack = textmate.INITIAL;
    for (const line of fs.readFileSync(path.join(directory, filename), 'utf8').split('\n')) {
      const result = grammar.tokenizeLine(line, stack); stack = result.ruleStack;
      assert.ok(!result.tokens.some(token => token.scopes.includes('invalid.illegal.escape.zhex')), filename);
    }
  }
  const configuration = JSON.parse(fs.readFileSync(path.join(root, 'language-configuration.json'), 'utf8'));
  assert.equal(configuration.comments.lineComment, '#');
  const indent = new RegExp(configuration.indentationRules.increaseIndentPattern);
  assert.ok(indent.test('word square(n: Int) -> Int'));
  assert.ok(indent.test('flow 4'));
  assert.ok(indent.test('let n = text.to-int(s) catch problem'));
  assert.ok(indent.test('text.to-int(s) catch'));
  assert.ok(!indent.test('if-ready()'));
});
