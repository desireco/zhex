// Docs sidebar search: one static JSON, ranked in the browser. Title and
// heading prefix/substring matches rank above body text; arrow keys move
// through the results and Enter opens the selection.
const LIMIT = 8;

// Promise-cached index loads; eviction on rejection keeps a failed load from
// poisoning the cache.
const indexCache = new Map();
const loadIndex = url => indexCache.get(url) ?? (() => {
  const pending = fetch(url).then(response => {
    if (!response.ok) throw new Error(`search index ${response.status}`);
    return response.json();
  });
  indexCache.set(url, pending);
  pending.catch(() => indexCache.delete(url));
  return pending;
})();

function best(page, q) {
  let best;
  const consider = (score, url, label, where) => {
    if (!best || score > best.score) best = { score, url, label, where };
  };
  const title = page.title.toLowerCase();
  if (title.startsWith(q)) consider(100, page.url, page.title, page.section);
  else if (title.includes(q)) consider(80, page.url, page.title, page.section);
  for (const [slug, text] of page.headings) {
    const heading = text.toLowerCase();
    if (heading.startsWith(q)) consider(60, `${page.url}#${slug}`, text, `${page.section} · ${page.title}`);
    else if (heading.includes(q)) consider(40, `${page.url}#${slug}`, text, `${page.section} · ${page.title}`);
  }
  if (page.text.includes(q)) consider(20, page.url, page.title, page.section);
  return best;
}

function wire(box) {
  const input = box.querySelector('.docs-search-input');
  const results = box.querySelector('.docs-search-results');
  const status = box.querySelector('.docs-search-status');
  let pages = null;
  let matches = [];

  const render = () => {
    const q = input.value.trim().toLowerCase();
    matches = q.length < 2 || !pages ? [] : pages.map(p => best(p, q)).filter(Boolean).sort((a, b) => b.score - a.score).slice(0, LIMIT);
    results.replaceChildren(...matches.map(m => {
      const li = document.createElement('li');
      const link = document.createElement('a');
      link.href = m.url;
      const label = document.createElement('strong');
      label.textContent = m.label;
      const where = document.createElement('span');
      where.className = 'where';
      where.textContent = m.where;
      link.append(label, where);
      li.append(link);
      return li;
    }));
    results.hidden = matches.length === 0;
    status.textContent = q.length < 2 || !pages ? '' : matches.length ? `${matches.length} result${matches.length === 1 ? '' : 's'}` : 'No matches in the docs.';
  };

  input.addEventListener('input', async () => {
    if (!pages) {
      try {
        pages = await loadIndex(box.dataset.index);
      } catch {
        status.textContent = 'Search is unavailable.';
        input.disabled = true;
        return;
      }
    }
    render();
  });

  input.addEventListener('keydown', event => {
    const first = results.querySelector('a');
    if (event.key === 'ArrowDown' && first) { event.preventDefault(); first.focus(); }
    else if (event.key === 'Enter' && matches.length) location.href = matches[0].url;
    else if (event.key === 'Escape') { input.value = ''; render(); }
  });

  results.addEventListener('keydown', event => {
    const links = [...results.querySelectorAll('a')];
    const at = links.indexOf(document.activeElement);
    if (event.key === 'ArrowDown') { event.preventDefault(); links[(at + 1) % links.length].focus(); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); at <= 0 ? input.focus() : links[at - 1].focus(); }
    else if (event.key === 'Escape') { input.value = ''; render(); input.focus(); }
  });
}

for (const box of document.querySelectorAll('.docs-search')) wire(box);
