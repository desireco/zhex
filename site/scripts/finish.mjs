import { cp, readFile, readdir, writeFile } from 'node:fs/promises';
import path from 'node:path';
const out = '../_site';
for (const name of ['styles.css', 'favicon.svg', 'logo.svg', 'llms.txt', 'design']) await cp(name, path.join(out, name), { recursive: true });
for (const [source, target] of [['../ROADMAP.md', 'ROADMAP.md'], ['../docs/manifesto.md', 'manifesto.md'], ['../examples', 'examples']]) await cp(source, path.join(out, target), { recursive: true });
await writeFile(path.join(out, '.nojekyll'), '');
async function files(dir) {
  const entries = await readdir(dir, { withFileTypes: true });
  return (await Promise.all(entries.map(e => e.isDirectory() ? files(path.join(dir, e.name)) : path.join(dir, e.name)))).flat();
}
const contents = new Map(await Promise.all((await files(out)).map(async file => [path.relative(out, file).split(path.sep).join('/'), await readFile(file, 'utf8')])));
for (const [name, html] of contents) {
  if (!name.endsWith('.html')) continue;
  for (const match of html.matchAll(/(?:href|src)=["']([^"']+)["']/g)) {
    const raw = match[1].replaceAll('&amp;', '&');
    if (/^(?:[a-z][a-z\d+.-]*:|\/\/)/i.test(raw)) continue;
    const url = new URL(raw, `https://example.com/zhex/${name}`);
    if (!url.pathname.startsWith('/zhex/')) throw new Error(`Link escapes Pages base in ${name}: ${raw}`);
    let target = decodeURIComponent(url.pathname.slice(6));
    if (!target || target.endsWith('/')) target += 'index.html';
    const linked = contents.get(target);
    if (linked === undefined) throw new Error(`Broken local link in ${name}: ${raw}`);
    if (url.hash && !linked.includes(`id="${decodeURIComponent(url.hash.slice(1))}"`) && !linked.includes(`id='${decodeURIComponent(url.hash.slice(1))}'`)) throw new Error(`Missing anchor in ${name}: ${raw}`);
  }
}
