import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
const read = file => readFile(`../_site/${file}`, 'utf8');
const data = JSON.parse(await readFile('data.json', 'utf8'));
const examples = await read('examples.html');
for (const example of data.Examples) {
  assert.equal(await read(example.SourceURL), example.Source, `${example.ID} download`);
  if (!['composition', 'factorial'].includes(example.ID)) {
    assert(examples.includes(`id="${example.ID}"`), `${example.ID} anchor`);
    assert(examples.includes(example.Command), `${example.ID} command`);
    assert(examples.includes(example.Highlighted), `${example.ID} highlighted source`);
  }
  for (const module of example.Modules || []) {
    assert.equal(await read(module.URL), module.Source, `${module.URL} download`);
    assert(examples.includes(module.Highlighted), `${module.URL} source`);
  }
}
const errors = await read('errors.html');
for (const entry of data.Catalog) assert(errors.includes(`id="${entry.Code}"`), `${entry.Code} anchor`);
const docs = await read('docs.html');
assert(docs.includes('<h1>Docs</h1>'), 'documentation landing heading');
assert(docs.includes('docs/getting-started.html'), 'getting started reachable from docs');
const home = await read('index.html');
const header = home.split('</header>')[0];
assert(header.includes('/zhex/docs.html'), 'main navigation links to Docs');
assert(!header.includes('Get started'), 'main navigation has one documentation entry');
assert(home.includes('id="start"'), 'legacy home quickstart anchor');
const gettingStarted = await read('docs/getting-started.html');
assert(gettingStarted.includes('docs-nav-mobile') && gettingStarted.includes('<details'), 'mobile documentation disclosure');
assert(gettingStarted.includes('aria-current="page"'), 'current documentation topic');
for (const id of ['build-and-run', 'read-the-program', 'try-the-repl', 'where-to-go-next']) assert(gettingStarted.includes(`id="${id}"`), `${id} legacy tutorial anchor`);
for (const id of ['features-title', 'modules-title', 'errors-title', 'setup-title', 'editors-title']) assert(docs.includes(`id="${id}"`), `${id} legacy anchor`);
for (const filename of ['index.html', 'roadmap.html', 'manifesto.html', 'llms.txt', 'design/design-system.html']) assert(await read(filename));
for (const file of await (await import('node:fs/promises')).readdir('../_site/docs')) {
  if (!file.endsWith('.html')) continue;
  const page = await read(`docs/${file}`);
  assert.equal((page.match(/<h1(?:\s|>)/g) || []).length, 1, `${file} must have one page heading`);
}
const roadmap = await read('roadmap.html');
for (const cssClass of ['roadmap-section done', 'roadmap-section next']) assert(roadmap.includes(cssClass), `${cssClass} styling`);
