import { For, Show, createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { track } from '../lib/analytics';
import { clearHistory, getPersonalization, preferredTags } from '../lib/personalization';
import { setSeo } from '../lib/seo';
import { VideoCard } from '../ui/VideoCard';
import { s } from '../ui/styles';

export function LibraryPage() {
  const [state, setState] = createSignal(getPersonalization());

  onSettled(() => {
    setSeo({
      title: 'Your library · PlantingTulips',
      description: 'Private browser-local favorites, recent views, and preferred tags.',
      canonicalPath: '/library',
      robots: 'noindex,nofollow,noarchive',
    });
    track('page_view', { metadata: { surface: 'library' } });
  });

  const clear = () => {
    clearHistory();
    setState(getPersonalization());
  };

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.contentHeader)}>
        <div><h1 {...stylex.props(s.heading)}>Your library</h1><p {...stylex.props(s.subheading)}>Stored only in this browser. No account is required.</p></div>
      </div>

      <Show when={preferredTags().length}>
        <section {...stylex.props(s.panel)} style={{ 'margin-bottom': '20px' }}>
          <h2 {...stylex.props(s.sectionTitle)}>Preferred tags</h2>
          <div {...stylex.props(s.chips)}><For each={preferredTags()}>{(tag) => <a href={`/tag/${slug(tag)}`} {...stylex.props(s.chip)}>{tag}</a>}</For></div>
        </section>
      </Show>

      <section style={{ 'margin-bottom': '28px' }}>
        <h2 {...stylex.props(s.heading)}>Favorites</h2>
        <Show when={state().favorites.length} fallback={<div {...stylex.props(s.panel)}>No favorites yet.</div>}>
          <div {...stylex.props(s.grid)}><For each={state().favorites}>{(video) => <VideoCard video={video} context="library" />}</For></div>
        </Show>
      </section>

      <section>
        <div {...stylex.props(s.contentHeader)}><div><h2 {...stylex.props(s.heading)}>Recently viewed</h2><p {...stylex.props(s.subheading)}>Up to 50 recent items are kept locally.</p></div><button type="button" onClick={clear} {...stylex.props(s.secondaryButton)}>Clear history</button></div>
        <Show when={state().recent.length} fallback={<div {...stylex.props(s.panel)}>No viewing history yet.</div>}>
          <div {...stylex.props(s.grid)}><For each={state().recent}>{(video) => <VideoCard video={video} context="library" />}</For></div>
        </Show>
      </section>
    </main>
  );
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}
