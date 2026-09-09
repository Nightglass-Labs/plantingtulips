export const categoryDirectory = [
  { slug: 'amateur', name: 'Amateur', query: 'amateur blowjob', description: 'Community-style and independently produced results.' },
  { slug: 'pov', name: 'POV', query: 'POV blowjob', description: 'A dedicated point-of-view index instead of a buried tag.' },
  { slug: 'deepthroat', name: 'Deepthroat', query: 'deepthroat', description: 'A focused subcategory with its own newest and top views.' },
  { slug: 'compilations', name: 'Compilations', query: 'blowjob compilation', description: 'Compilation-focused browsing.' },
  { slug: 'long-form', name: 'Long-form', query: 'long blowjob', description: 'Longer videos grouped into one durable hub.' },
  { slug: 'short', name: 'Short', query: 'short blowjob', description: 'Short-form videos for quick browsing.' },
] as const;

export function categoryBySlug(slug: string) {
  return categoryDirectory.find((category) => category.slug === slug);
}
