import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
const read = file => readFile(`../_site/${file}`, 'utf8');
const data = JSON.parse(await readFile('data.json', 'utf8'));
const examples = await read('examples.html');
const shownSources = new Set(data.Examples.flatMap(e => [e.SourceURL, ...(e.Modules || []).map(m => m.URL)]));
// Tools that live beside the examples but need command-line arguments, so the
// site cannot execute them; they run in TestExamples instead of at build time.
const argTools = new Set(['filter.zhex']);
// Files under demos/ are demos-page material, not examples-page cards: each
// one must be a demo manifest row (id naming the file) instead.
for (const file of await readdir('../examples', { recursive: true })) {
  if (!file.endsWith('.zhex') || argTools.has(file)) continue;
  if (file.startsWith('demos/')) assert(data.Demos.some(d => file === `demos/${d.ID}.zhex`), `${file} must be a demo manifest row`);
  else assert(shownSources.has(`examples/${file}`), `${file} must appear on the examples page`);
}
assert(!/<details\b/i.test(examples), 'examples must be visible without expanding');
for (const example of data.Examples) {
  assert.equal(await read(example.SourceURL), example.Source, `${example.ID} download`);
  assert(examples.includes(`id="${example.ID}"`), `${example.ID} anchor`);
  assert(examples.includes(example.Command), `${example.ID} command`);
  assert(examples.includes(example.Highlighted), `${example.ID} highlighted source`);
  for (const module of example.Modules || []) {
    assert.equal(await read(module.URL), module.Source, `${module.URL} download`);
    assert(examples.includes(module.Highlighted), `${module.URL} source`);
  }
}
const errors = await read('errors.html');
for (const entry of data.Catalog) assert(errors.includes(`id="${entry.Code}"`), `${entry.Code} anchor`);
// #120: the docs landing retired — docs.html is a redirect to TL;DR.
const docs = await read('docs.html');
assert(docs.includes('http-equiv="refresh"') && docs.includes('docs/tldr.html'), 'docs.html redirects to TL;DR');
assert(!docs.includes('<h1>Docs</h1>'), 'no landing content on docs.html');
const home = await read('index.html');
const header = home.split('</header>')[0];
assert(header.includes('/zhex/docs/tldr.html'), 'main navigation links to Docs');
assert(!header.includes('Get started'), 'main navigation has one documentation entry');
assert(home.includes('id="start"'), 'legacy home quickstart anchor');
const gettingStarted = await read('docs/getting-started.html');
assert(gettingStarted.includes('docs-nav-mobile') && gettingStarted.includes('<details'), 'mobile documentation disclosure');
assert(gettingStarted.includes('aria-current="page"'), 'current documentation topic');
for (const id of ['build-and-run', 'read-the-program', 'try-the-repl', 'where-to-go-next']) assert(gettingStarted.includes(`id="${id}"`), `${id} legacy tutorial anchor`);
for (const filename of ['index.html', 'roadmap.html', 'changelog.html', 'manifesto.html', 'demos.html', 'advent-of-code.html', 'playground.html', 'llms.txt', 'design/design-system.html']) assert(await read(filename));
for (const file of ['index.html', 'roadmap.html', 'manifesto.html', 'examples.html', 'demos.html', 'advent-of-code.html', 'playground.html', 'errors.html']) assert((await read(file)).includes('class="accent-rule"'), `${file} header accent rule`);
// #115: the demos page carries the demo players, and the examples page
// points to it from the top instead of holding the section itself.
const demos = await read('demos.html');
assert(demos.includes('class="demo-player"'), 'demos page carries the demo players');
assert(examples.includes('href="demos.html"'), 'examples page links to the demos page');
assert(!examples.includes('demo-player'), 'the demo players moved off the examples page');
for (const file of await (await import('node:fs/promises')).readdir('../_site/docs')) {
  if (!file.endsWith('.html')) continue;
  const page = await read(`docs/${file}`);
  assert.equal((page.match(/<h1(?:\s|>)/g) || []).length, 1, `${file} must have one page heading`);
  assert(page.includes('class="accent-rule"'), `${file} header accent rule`);
}
const search = JSON.parse(await read('search-index.json'));
const docPages = (await readdir('../_site/docs')).filter(f => f.endsWith('.html'));
assert.equal(search.length, docPages.length, 'search index covers every docs page');
for (const entry of search) {
  assert(docPages.includes(`${entry.slug}.html`), `search index entry ${entry.slug} matches a docs page`);
  assert(entry.title && entry.section && entry.url.endsWith(`docs/${entry.slug}.html`), `${entry.slug} search fields`);
  assert(entry.text.length > 100, `${entry.slug} search body text`);
  const page = await read(`docs/${entry.slug}.html`);
  assert(page.includes('docs-search-input'), `${entry.slug} sidebar search box`);
  for (const [slug] of entry.headings) assert(page.includes(`id="${slug}"`), `${entry.slug} search heading anchor ${slug}`);
}
const roadmap = await read('roadmap.html');
for (const cssClass of ['roadmap-section done', 'roadmap-section next']) assert(roadmap.includes(cssClass), `${cssClass} styling`);
// #126: the AoC page renders one door per Aoc row, both year tabs, and the
// exact honest status labels; no door count is hardcoded.
const aoc = await read('advent-of-code.html');
assert.equal((aoc.match(/class="aoc-door"/g) || []).length, data.Aoc.length, 'one door per AoC day');
assert(aoc.includes('Starts December 1'), '2026 tab teaser');
assert(aoc.includes('aria-selected="true"') && aoc.includes('aria-selected="false"'), 'both year tabs render');
if (data.Aoc.some((day) => day.Status === 'empty')) assert(aoc.includes('Not solved yet'), 'honest empty status label');
assert(aoc.includes('Show solution'), 'solution reveal per day');
for (const day of data.Aoc) assert(aoc.includes(`id="aoc-day-${day.Year}-${day.Day}"`) && aoc.includes(day.OfficialURL), `day ${day.Year}-${day.Day} view`);
// #137: the years render from the data — one tab and panel per archive
// year, plus the teaser year after the newest.
const years = [...new Set(data.Aoc.map(day => day.Year))];
for (const year of years) {
  assert(aoc.includes(`id="aoc-tab-${year}"`) && aoc.includes(`id="aoc-panel-${year}"`), `year ${year} tab and panel render`);
  assert.equal((aoc.match(new RegExp(`aria-label="${year} doors"`, "g")) || []).length, 1, `year ${year} calendar renders`);
}
const teaserYear = years[years.length - 1] + 1;
assert(aoc.includes(`id="aoc-tab-${teaserYear}"`), `teaser year ${teaserYear} tab renders`);
// #128: the page mounts a run panel exactly on wasm-backed doors and the
// fence note exactly on fenced ones — empty and working doors render
// neither, so the counts must track the data.
assert.equal((aoc.match(/class="run-panel aoc-run-panel"/g) || []).length, data.Aoc.filter(d => d.Wasm).length, 'a run panel per wasm-backed door');
assert.equal((aoc.match(/Not compiled to WebAssembly yet/g) || []).length, data.Aoc.filter(d => d.WasmNote).length, 'a fence note per fenced door');
// #88: every example card links into the playground by id, and the
// playground page carries the cover, the two modes, and the tour.
const playground = await read('playground.html');
for (const example of data.Examples) {
  assert(examples.includes(`playground.html?example=${example.ID}`), `${example.ID} links to the playground`);
  // The index rides in as escaped JSON inside the define:vars script, so the
  // bare id is the stable marker.
  assert(playground.includes(example.ID), `${example.ID} is in the playground's example index`);
}
for (const marker of ['pg-cover', 'Open playground', 'Loading Zhex…', 'role="tablist"', 'Run the tour', 'Program input', 'Copy link']) {
  assert(playground.includes(marker), `playground carries ${marker}`);
}
assert(playground.includes('reports end-of-file'), 'playground documents the stdin boundary honestly');
const nav = home.split('</header>')[0];
assert(nav.includes('advent-of-code.html'), 'main navigation links to the AoC page');
assert(nav.includes('playground.html'), 'main navigation links to the playground');
