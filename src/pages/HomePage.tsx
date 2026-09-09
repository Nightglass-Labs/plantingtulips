import { useSearchParams } from '@solidjs/router';
import { Errored, For, Loading, Show, createEffect, createMemo } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { searchVideos } from '../lib/api';
import { track } from '../lib/analytics';
import { categoryDirectory } from '../lib/categories';
import { preferredTags } from '../lib/personalization';
import { setSeo } from '../lib/seo';
import { AdSlot } from '../ui/AdSlot';
import { VideoCard } from '../ui/VideoCard';
import { s } from '../ui/styles';

const quickSearches = ['blowjob', 'POV', 'amateur', 'deepthroat', 'compilation', 'professional', 'long', 'short'];

export function HomePage() {
  const [params] = useSearchParams();
  const query = () => first(params.q, 'blowjob');
  const page = () => Math.max(1, Number(first(params.page, '1')) || 1);
  const sort = () => first(params.sort, 'top-weekly');
  const catalog = createMemo(() => searchVideos(query(), page(), sort()));
  const personalTags = createMemo(() => preferredTags(6));

  createEffect(
    () => ({ query: query(), sort: sort(), page: page() }),
    (state) => {
      setSeo({
        title: state.query === 'blowjob' ? 'PlantingTulips · video discovery' : `${state.query} videos · PlantingTulips`,
        description: 'A focused video discovery catalog built from approved provider APIs and embeds.',
        canonicalPath: state.query === 'blowjob' ? '/' : `/?q=${encodeURIComponent(state.query)}`,
        keywords: [state.query, 'video discovery', 'PlantingTulips'],
      });
      track('search', { query: state.query, metadata: { sort: state.sort, page: state.page } });
    },
  );

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.columns)}>
        <aside {...stylex.props(s.leftRail)}>
          <div {...stylex.props(s.railBox)}>
            <h2 {...stylex.props(s.sectionTitle)}>Browse</h2>
            <a href="/?q=blowjob" {...stylex.props(s.railLink)}>Featured</a>
            <a href="/?q=blowjob&sort=latest" {...stylex.props(s.railLink)}>Newest</a>
            <a href="/?q=blowjob&sort=most-popular" {...stylex.props(s.railLink)}>Most viewed</a>
            <a href="/?q=blowjob&sort=top-rated" {...stylex.props(s.railLink)}>Top rated</a>
            <a href="/categories" {...stylex.props(s.railLink)}>All categories</a>
            <a href="/library" {...stylex.props(s.railLink)}>Your library</a>
          </div>
          <div {...stylex.props(s.railBox)}>
            <h2 {...stylex.props(s.sectionTitle)}>Prefixes / tags</h2>
            <For each={quickSearches}>{(term) => <a href={`/tag/${slug(term)}`} {...stylex.props(s.railLink)}>{term}</a>}</For>
          </div>
        </aside>

        <section>
          <div {...stylex.props(s.contentHeader)}>
            <div>
              <h1 {...stylex.props(s.heading)}>{query()}</h1>
              <p {...stylex.props(s.subheading)}>Persistent catalog results with approved provider APIs as live fallback.</p>
            </div>
          </div>
          <div {...stylex.props(s.chips)}><For each={quickSearches}>{(term) => <a href={`/?q=${encodeURIComponent(term)}`} {...stylex.props(s.chip)}>{term}</a>}</For></div>
          <Show when={personalTags().length > 0}>
            <div {...stylex.props(s.panel)} style={{ 'margin-top': '12px' }}><strong>Based on this browser</strong><div {...stylex.props(s.chips)}><For each={personalTags()}>{(value) => <a href={`/tag/${slug(value)}`} {...stylex.props(s.chip)}>{value}</a>}</For></div></div>
          </Show>
          <AdSlot slot="feed-banner" />

          <div {...stylex.props(s.directory)}>
            <div {...stylex.props(s.directoryHead)}><div><strong>Browse sections</strong><div {...stylex.props(s.subheading)}>Durable category hubs backed by the normalized catalog.</div></div><a href="/categories" {...stylex.props(s.railLink)}>View directory →</a></div>
            <div {...stylex.props(s.directoryGrid)}>
              <For each={categoryDirectory.slice(0, 4)}>{(section) => (
                <a href={`/category/${section.slug}`} {...stylex.props(s.directoryItem)}>
                  <div><div {...stylex.props(s.directoryTitle)}>{section.name}</div><div {...stylex.props(s.directoryDescription)}>{section.description}</div></div>
                  <span {...stylex.props(s.directoryBadge)}>hub</span>
                </a>
              )}</For>
            </div>
          </div>

          <Errored fallback={(error) => <div {...stylex.props(s.panel)}>Catalog unavailable: {String(error())}</div>}>
            <Loading fallback={<div {...stylex.props(s.panel)}>Loading catalog…</div>}>
              <Catalog data={catalog()} sort={sort()} />
            </Loading>
          </Errored>
        </section>

        <aside {...stylex.props(s.rightRail)}>
          <AdSlot slot="sidebar" />
          <Loading fallback={<div {...stylex.props(s.railBox)}>Checking providers…</div>}><ProviderRail data={catalog()} /></Loading>
          <div {...stylex.props(s.railBox)}>
            <h2 {...stylex.props(s.sectionTitle)}>Catalog mode</h2>
            <p {...stylex.props(s.subheading)}>When D1 has matching records the site reads its own catalog. Provider APIs remain a live fallback for cold or missing searches.</p>
          </div>
        </aside>
      </div>
    </main>
  );
}

function Catalog(props: { data: Awaited<ReturnType<typeof searchVideos>>; sort: string }) {
  return (
    <>
      <div {...stylex.props(s.contentHeader)}><div><h2 {...stylex.props(s.heading)}>Latest results</h2><p {...stylex.props(s.subheading)}>{props.data.totalCount.toLocaleString()} results · {props.data.source === 'catalog' ? 'PlantingTulips catalog' : 'live provider fallback'}</p></div></div>
      <div {...stylex.props(s.grid)}><For each={props.data.videos}>{(video) => <VideoCard video={video} />}</For></div>
      <div {...stylex.props(s.buttonRow)} style={{ 'margin-top': '24px' }}>
        {props.data.page > 1 && <a href={`/?q=${encodeURIComponent(props.data.query)}&sort=${encodeURIComponent(props.sort)}&page=${props.data.page - 1}`} {...stylex.props(s.secondaryButton)}>Previous</a>}
        {props.data.page < props.data.totalPages && <a href={`/?q=${encodeURIComponent(props.data.query)}&sort=${encodeURIComponent(props.sort)}&page=${props.data.page + 1}`} {...stylex.props(s.secondaryButton)}>Next</a>}
      </div>
    </>
  );
}

function ProviderRail(props: { data: Awaited<ReturnType<typeof searchVideos>> }) {
  return (
    <div {...stylex.props(s.railBox)}>
      <h2 {...stylex.props(s.sectionTitle)}>Providers</h2>
      <For each={Object.entries(props.data.providers)}>{([name, provider]) => <div {...stylex.props(s.status)} title={provider.note}><span>{name}</span><span {...stylex.props(provider.enabled ? s.dotOn : s.dotOff)} /></div>}</For>
    </div>
  );
}

function first(value: string | string[] | undefined, fallback: string) {
  return Array.isArray(value) ? value[0] ?? fallback : value ?? fallback;
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}
