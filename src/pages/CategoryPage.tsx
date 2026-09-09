import { useParams, useSearchParams } from '@solidjs/router';
import { Errored, For, Loading, createEffect, createMemo } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { searchVideos } from '../lib/api';
import { track } from '../lib/analytics';
import { categoryBySlug } from '../lib/categories';
import { setSeo } from '../lib/seo';
import { AdSlot } from '../ui/AdSlot';
import { VideoCard } from '../ui/VideoCard';
import { s } from '../ui/styles';

export function CategoryPage() {
  const params = useParams();
  const [search] = useSearchParams();
  const category = () => categoryBySlug(params.slug || '');
  const page = () => Math.max(1, Number(first(search.page, '1')) || 1);
  const sort = () => first(search.sort, 'top-weekly');
  const catalog = createMemo(async () => {
    const current = category();
    if (!current) throw new Error('Unknown category');
    return searchVideos(current.query, page(), sort());
  });

  createEffect(
    () => category(),
    (current) => {
      if (!current) return;
      setSeo({
        title: `${current.name} videos · PlantingTulips`,
        description: current.description,
        canonicalPath: `/category/${current.slug}`,
        keywords: [current.name, current.query, 'PlantingTulips'],
      });
      track('category_view', { category: current.slug, query: current.query });
    },
  );

  return (
    <main {...stylex.props(s.page)}>
      <Errored fallback={(error) => <div {...stylex.props(s.panel)}>Unable to load category: {String(error())}</div>}>
        <Loading fallback={<div {...stylex.props(s.panel)}>Loading category…</div>}>
          <Category data={catalog()} category={category()!} sort={sort()} />
        </Loading>
      </Errored>
    </main>
  );
}

function Category(props: { data: Awaited<ReturnType<typeof searchVideos>>; category: NonNullable<ReturnType<typeof categoryBySlug>>; sort: string }) {
  const sorts = [
    ['top-weekly', 'Top'], ['latest', 'Newest'], ['most-popular', 'Most viewed'], ['top-rated', 'Top rated'], ['longest', 'Longest'],
  ];
  return (
    <section>
      <a href="/categories" {...stylex.props(s.railLink)}>← All categories</a>
      <div {...stylex.props(s.contentHeader)}>
        <div>
          <h1 {...stylex.props(s.heading)}>{props.category.name}</h1>
          <p {...stylex.props(s.subheading)}>{props.category.description} · {props.data.totalCount.toLocaleString()} results</p>
        </div>
      </div>
      <div {...stylex.props(s.chips)}>
        <For each={sorts}>{([value, label]) => <a href={`/category/${props.category.slug}?sort=${value}`} {...stylex.props(s.chip)}>{label}</a>}</For>
      </div>
      <AdSlot slot="feed-banner" />
      <div {...stylex.props(s.grid)}><For each={props.data.videos}>{(video) => <VideoCard video={video} />}</For></div>
      <div {...stylex.props(s.buttonRow)} style={{ 'margin-top': '24px' }}>
        {props.data.page > 1 && <a href={`/category/${props.category.slug}?sort=${encodeURIComponent(props.sort)}&page=${props.data.page - 1}`} {...stylex.props(s.secondaryButton)}>Previous</a>}
        {props.data.page < props.data.totalPages && <a href={`/category/${props.category.slug}?sort=${encodeURIComponent(props.sort)}&page=${props.data.page + 1}`} {...stylex.props(s.secondaryButton)}>Next</a>}
      </div>
    </section>
  );
}

function first(value: string | string[] | undefined, fallback: string) {
  return Array.isArray(value) ? value[0] ?? fallback : value ?? fallback;
}
