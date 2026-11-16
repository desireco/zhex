import { getCollection, render } from 'astro:content';

// Build-time search index over the docs collection, emitted as one static
// JSON the docs sidebar fetches. Regenerated on every build, so it can never
// go stale; the markdown stays the single source of truth.
function plainText(markdown) {
  return markdown
    .replace(/<!--[\s\S]*?-->/g, ' ')
    .replace(/<[^>]+>/g, ' ')
    .replace(/```[\w-]*/g, ' ')
    .replace(/!\[([^\]]*)\]\(([^)]*)\)/g, '$1')
    .replace(/\[([^\]]*)\]\(([^)]*)\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s{0,3}>\s?/gm, '')
    .replace(/[*_`~]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

export async function GET() {
  const entries = (await getCollection('docs')).sort((a, b) => a.data.order - b.data.order);
  const base = import.meta.env.BASE_URL.replace(/\/$/, '');
  const pages = [];
  for (const entry of entries) {
    const { headings } = await render(entry);
    pages.push({
      slug: entry.id,
      title: entry.data.title,
      section: entry.data.section,
      url: `${base}/docs/${entry.id}.html`,
      headings: headings.filter(h => h.depth === 2 || h.depth === 3).map(h => [h.slug, h.text]),
      text: plainText(`${entry.data.description}
${entry.body}`),
    });
  }
  return new Response(JSON.stringify(pages), { headers: { 'Content-Type': 'application/json' } });
}
