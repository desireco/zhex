// Shared inline renderer for roadmap text and the row around it (roadmap and
// changelog pages):
// backticks become code spans, and repo doc paths (docs/*.md) link to their
// source on GitHub. Escaping happens first; one combined pass keeps inserted
// markup out of later matches.
const REPO = "https://github.com/desireco/zhex/blob/main";

const escapeHtml = (text) => text
  .replace(/&/g, "&amp;")
  .replace(/</g, "&lt;")
  .replace(/>/g, "&gt;")
  .replace(/"/g, "&quot;");

const isDocPath = (text) => /^docs\/[a-z0-9-]+\.md$/.test(text);

const docLink = (path, inner) => `<a href="${REPO}/${path}" target="_blank" rel="noopener">${inner}</a>`;

export function inline(text) {
  return escapeHtml(text).replace(/`([^`]+)`|(docs\/[a-z0-9-]+\.md)/g, (_, code, doc) => {
    if (code !== undefined) {
      return isDocPath(code) ? docLink(code, `<code>${code}</code>`) : `<code>${code}</code>`;
    }
    return docLink(doc, doc);
  });
}

// One roadmap or changelog row: the optional lead in bold with its ": ", the
// inline-rendered text, then the facts list under it. The lead goes through
// Astro's own text escaping, apostrophes included, so it renders exactly as
// the {item.Lead} text node it replaces.
const escapeText = (text) => escapeHtml(text).replace(/'/g, '&#39;');

export function row(item) {
  const facts = item.facts ?? [];
  return `${item.Lead ? `<strong>${escapeText(item.Lead)}</strong>: ` : ''}${inline(item.Text)}`
    + (facts.length > 0 ? `<ul class="facts">${facts.map(fact => `<li>${inline(fact)}</li>`).join('')}</ul>` : '');
}
