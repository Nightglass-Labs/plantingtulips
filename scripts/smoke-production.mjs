const baseInput = process.env.SITE_URL || process.argv[2];
if (!baseInput) {
  console.error('SITE_URL or a base URL argument is required');
  process.exit(2);
}

const requireCatalog = process.env.REQUIRE_CATALOG === '1';
const requireGrowth = process.env.REQUIRE_GROWTH === '1';
const base = new URL(baseInput.endsWith('/') ? baseInput : `${baseInput}/`);
const api = '/api/v1';

async function request(path, options = {}) {
  const url = new URL(path.replace(/^\//, ''), base);
  const response = await fetch(url, {
    ...options,
    headers: { accept: 'application/json, text/html;q=0.8', ...(options.headers || {}) },
    signal: AbortSignal.timeout(15_000),
  });
  if (!response.ok) throw new Error(`${response.status} ${response.statusText} for ${url}`);
  return response;
}

async function json(path) {
  return request(path).then((response) => response.json());
}

console.log(`Smoke testing ${base.origin}${requireCatalog ? ' (catalog required)' : ''}${requireGrowth ? ' (growth required)' : ''}`);

const health = await json(`${api}/healthz`);
if (health.ok !== true || health.database !== 'ok') throw new Error('Go health check did not confirm Turso connectivity');
console.log('✓ Go/Turso health');

const home = await request('/');
const homeText = await home.text();
if (!homeText.includes('root')) throw new Error('Homepage did not contain the SPA root');
if (requireGrowth && home.headers.get('x-pt-seo') !== 'go') throw new Error('Homepage did not pass through Go SEO rendering');
console.log('✓ embedded SPA homepage');

if (requireGrowth) {
  const categoriesPage = await request('/categories');
  const categoriesHtml = await categoriesPage.text();
  if (categoriesPage.headers.get('x-pt-seo') !== 'go') throw new Error('Category directory did not receive Go SEO metadata');
  if (!categoriesHtml.includes('<link rel="canonical"')) throw new Error('Category directory did not include a canonical link');
  if (!categoriesHtml.includes('name="robots" content="index,follow')) throw new Error('Category directory was not indexable in initial HTML');

  for (const privatePath of ['/admin', '/library', '/report']) {
    const page = await request(privatePath);
    const pageHtml = await page.text();
    if (!pageHtml.includes('name="robots" content="noindex,nofollow,noarchive"')) throw new Error(`${privatePath} was not noindex in initial HTML`);
  }
  console.log('✓ Go SEO + private-route noindex');
}

const robotsText = await request('/robots.txt').then((response) => response.text());
if (!robotsText.includes(`Sitemap: ${base.origin}/sitemap.xml`)) throw new Error('robots.txt did not advertise the deployed sitemap origin');
console.log('✓ robots');

const sitemapText = await request('/sitemap.xml').then((response) => response.text());
if (!sitemapText.includes('<urlset')) throw new Error('sitemap.xml did not return an XML urlset');
console.log('✓ sitemap');

const categories = await json(`${api}/categories`);
if (!Array.isArray(categories.categories)) throw new Error(`${api}/categories did not return categories[]`);
if (requireCatalog && categories.configured !== true) throw new Error('Production Turso catalog is not configured');
console.log(`✓ categories (${categories.categories.length}, configured=${Boolean(categories.configured)})`);

const search = await json(`${api}/videos?q=all&page=1&sort=latest`);
if (!Array.isArray(search.videos)) throw new Error(`${api}/videos did not return videos[]`);
if (!search.providers || typeof search.providers !== 'object') throw new Error(`${api}/videos did not return provider status`);
if (requireCatalog && search.source !== 'catalog') throw new Error(`Expected Turso catalog source but got ${search.source || 'unknown'}`);
console.log(`✓ catalog/search (${search.videos.length} videos, source=${search.source || 'live'})`);

const analytics = await request(`${api}/events`, {
  method: 'POST',
  headers: { 'content-type': 'application/json' },
  body: JSON.stringify({ sessionId: `smoke-${Date.now()}`, eventName: 'page_view', path: '/__smoke__', metadata: { smoke: true } }),
});
if (requireCatalog && analytics.status !== 204) throw new Error(`Expected analytics persistence (204), got ${analytics.status}`);
console.log(`✓ analytics (${analytics.status})`);

const first = search.videos[0];
if (first?.provider && first?.id) {
  const provider = encodeURIComponent(first.provider);
  const id = encodeURIComponent(first.id);
  const detail = await json(`${api}/video/${provider}/${id}`);
  if (detail.id !== first.id) throw new Error('Video detail ID did not match search result');
  console.log(`✓ video detail (${first.provider}/${first.id})`);

  if (requireGrowth) {
    const watchPage = await request(`/watch/${provider}/${id}`);
    const watchHtml = await watchPage.text();
    if (watchPage.headers.get('x-pt-seo') !== 'go') throw new Error('Watch page did not receive Go SEO metadata');
    if (!watchHtml.includes('id="pt-server-video-jsonld"')) throw new Error('Watch page did not include server-rendered VideoObject JSON-LD');
    if (!watchHtml.includes('property="og:image"')) throw new Error('Watch page did not include server-rendered OpenGraph image');
    console.log('✓ watch-page Go metadata');
  }

  const related = await json(`${api}/related/${provider}/${id}`);
  if (!Array.isArray(related.videos)) throw new Error(`${api}/related did not return videos[]`);
  console.log(`✓ related videos (${related.videos.length})`);
} else if (requireCatalog) {
  throw new Error('Production Turso catalog is required but contains no searchable video');
} else {
  console.warn('! no searchable video was available; detail/related checks skipped');
}

console.log('Production smoke passed.');
