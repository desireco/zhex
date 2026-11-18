// Shared inline renderer for roadmap text (roadmap and changelog pages):
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
