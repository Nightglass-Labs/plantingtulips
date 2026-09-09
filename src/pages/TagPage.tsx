import { useParams, useSearchParams } from '@solidjs/router';
import { Errored, For, Loading, createMemo, createEffect } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { searchVideos } from '../lib/api';
import { track } from '../lib/analytics';
import { setSeo } from '../lib/seo';
import { AdSlot } from '../ui/AdSlot';
import { VideoCard } from '../ui/VideoCard';
import { s } from '../ui/styles';

export function TagPage() {
  const params = useParams();
  const [search] = useSearchParams();
  const tag = () => decodeURIComponent(params.slug || '').replace(/-/g, ' ').trim();
  const page = () => Math.max(1, Number(first(search.page, '1')) || 1);
  const sort = () => first(search.sort, 'top-weekly');
  const catalog = createMemo(() => searchVideos(tag(), page(), sort()));

  createEffect(
    () => ({ tag: tag(), slug: params.slug || '' }),
    (state) => {
      if (!state.tag) return;
      setSeo({
        title: `${titleCase(state.tag)} videos · PlantingTulips`,
        description: `Browse ${state.tag} videos across the PlantingTulips normalized catalog.`,
        canonicalPath: `/tag/${encodeURIComponent(state.slug)}`,
        keywords: [state.tag, 'video', 'PlantingTulips'],
      });
      track('page_view', { query: state.tag, metadata: { surface: 'tag' } });
    },
  );

  return (
    <main {...stylex.props(s.page)}>
      <Errored fallback={(error) => <div {...stylex.props(s.panel)}>Unable to load tag: {String(error())}</div>}>
        <Loading fallback={<div {...stylex.props(s.panel)}>Loading tag…</div>}>
          <section>
            <a href="/categories" {...stylex.props(s.railLink)}>← Browse directory</a>
            <div {...stylex.props(s.contentHeader)}><div><h1 {...stylex.props(s.heading)}>{titleCase(tag())}</h1><p {...stylex.props(s.subheading)}>{catalog().totalCount.toLocaleString()} catalog results</p></div></div>
            <AdSlot slot="feed-banner" />
            <div {...stylex.props(s.grid)}><For each={catalog().videos}>{(video) => <VideoCard video={video} />}</For></div>
            <div {...stylex.props(s.buttonRow)} style={{ 'margin-top': '24px' }}>
              {catalog().page > 1 && <a href={`/tag/${params.slug}?sort=${encodeURIComponent(sort())}&page=${catalog().page - 1}`} {...stylex.props(s.secondaryButton)}>Previous</a>}
              {catalog().page < catalog().totalPages && <a href={`/tag/${params.slug}?sort=${encodeURIComponent(sort())}&page=${catalog().page + 1}`} {...stylex.props(s.secondaryButton)}>Next</a>}
            </div>
          </section>
        </Loading>
      </Errored>
    </main>
  );
}

function first(value: string | string[] | undefined, fallback: string) {
  return Array.isArray(value) ? value[0] ?? fallback : value ?? fallback;
}

function titleCase(value: string) {
  return value.replace(/\b\w/g, (letter) => letter.toUpperCase());
}
