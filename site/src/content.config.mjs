import { defineCollection, z } from 'astro:content';
import { glob } from 'astro/loaders';

const docs = defineCollection({
  loader: glob({ base: './src/content/docs', pattern: '*.md' }),
  schema: z.object({ title: z.string(), description: z.string(), section: z.enum(['Start', 'Language', 'Standard library', 'Tools']), order: z.number() }),
});
export const collections = { docs };
