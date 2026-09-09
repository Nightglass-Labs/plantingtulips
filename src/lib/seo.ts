export function setSeo(input: {
  title: string;
  description: string;
  canonicalPath?: string;
  keywords?: string[];
  robots?: string;
  image?: string;
  type?: 'website' | 'video.other';
}) {
  if (typeof document === 'undefined') return;
  document.title = input.title;
  upsertMeta('description', input.description);
  upsertMeta('robots', input.robots || 'index,follow,max-image-preview:large');
  upsertMeta('og:title', input.title, 'property');
  upsertMeta('og:description', input.description, 'property');
  upsertMeta('og:type', input.type || 'website', 'property');
  if (input.keywords?.length) upsertMeta('keywords', input.keywords.join(','));
  else document.querySelector('meta[name="keywords"]')?.remove();

  if (input.image) upsertMeta('og:image', input.image, 'property');
  else document.querySelector('meta[property="og:image"]')?.remove();

  // Route transitions must not retain structured data from a previous watch page.
  document.getElementById('pt-video-jsonld')?.remove();
  document.getElementById('pt-server-video-jsonld')?.remove();

  const canonical = `${window.location.origin}${input.canonicalPath || window.location.pathname}`;
  let link = document.querySelector<HTMLLinkElement>('link[rel="canonical"]');
  if (!link) {
    link = document.createElement('link');
    link.rel = 'canonical';
    document.head.append(link);
  }
  link.href = canonical;
  upsertMeta('og:url', canonical, 'property');
}

export function setVideoStructuredData(input: {
  name: string;
  description: string;
  thumbnailUrl: string;
  embedUrl: string;
  contentUrl?: string;
}) {
  if (typeof document === 'undefined') return;
  const id = 'pt-video-jsonld';
  document.getElementById(id)?.remove();
  const script = document.createElement('script');
  script.id = id;
  script.type = 'application/ld+json';
  script.textContent = JSON.stringify({
    '@context': 'https://schema.org',
    '@type': 'VideoObject',
    name: input.name,
    description: input.description,
    thumbnailUrl: [input.thumbnailUrl],
    embedUrl: input.embedUrl,
    ...(input.contentUrl ? { contentUrl: input.contentUrl } : {}),
  });
  document.head.append(script);
}

function upsertMeta(name: string, content: string, attribute: 'name' | 'property' = 'name') {
  let node = document.querySelector<HTMLMetaElement>(`meta[${attribute}="${name}"]`);
  if (!node) {
    node = document.createElement('meta');
    node.setAttribute(attribute, name);
    document.head.append(node);
  }
  node.content = content;
}
