import { For, Loading, createMemo, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { getCategories } from '../lib/api';
import { categoryDirectory } from '../lib/categories';
import { setSeo } from '../lib/seo';
import { s } from '../ui/styles';

export function CategoriesPage() {
  const directory = createMemo(() => getCategories());

  onSettled(() => {
    setSeo({
      title: 'Browse categories · PlantingTulips',
      description: 'Browse durable category hubs across the PlantingTulips discovery catalog.',
      canonicalPath: '/categories',
    });
  });

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.contentHeader)}>
        <div><h1 {...stylex.props(s.heading)}>Browse categories</h1><p {...stylex.props(s.subheading)}>Persistent directory hubs instead of disposable search links.</p></div>
      </div>
      <Loading fallback={<div {...stylex.props(s.panel)}>Loading category directory…</div>}>
        <Directory data={directory()} />
      </Loading>
    </main>
  );
}

function Directory(props: { data: Awaited<ReturnType<typeof getCategories>> }) {
  const items = () => props.data.categories.length ? props.data.categories : categoryDirectory.map((category) => ({ ...category, videoCount: 0 }));
  return (
    <div {...stylex.props(s.directory)}>
      <div {...stylex.props(s.directoryGrid)}>
        <For each={items()}>{(category) => (
          <a href={`/category/${category.slug}`} {...stylex.props(s.directoryItem)}>
            <div>
              <div {...stylex.props(s.directoryTitle)}>{category.name}</div>
              <div {...stylex.props(s.directoryDescription)}>{category.description}</div>
            </div>
            <span {...stylex.props(s.directoryBadge)}>{category.videoCount ? category.videoCount.toLocaleString() : 'hub'}</span>
          </a>
        )}</For>
      </div>
    </div>
  );
}
