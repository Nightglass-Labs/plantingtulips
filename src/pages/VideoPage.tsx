import { useParams } from '@solidjs/router';
import { Errored, For, Loading, createMemo, createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { getRelatedVideos, getVideo } from '../lib/api';
import { track } from '../lib/analytics';
import { hideVideo, isFavorite, recordViewed, toggleFavorite } from '../lib/personalization';
import { setSeo, setVideoStructuredData } from '../lib/seo';
import { AdSlot } from '../ui/AdSlot';
import { VideoCard } from '../ui/VideoCard';
import { s } from '../ui/styles';

export function VideoPage() {
  const params = useParams();
  const video = createMemo(() => getVideo(params.provider ?? '', params.id ?? ''));
  const related = createMemo(() => getRelatedVideos(params.provider ?? '', params.id ?? ''));

  return (
    <main {...stylex.props(s.page)}>
      <Errored fallback={(error) => <div {...stylex.props(s.panel)}>Unable to load this video: {String(error())}</div>}>
        <Loading fallback={<div {...stylex.props(s.panel)}>Loading video…</div>}><Video data={video()} /></Loading>
      </Errored>
      <AdSlot slot="player-below" />
      <Loading fallback={<div {...stylex.props(s.panel)}>Finding related videos…</div>}><Related videos={related()} /></Loading>
    </main>
  );
}

function Video(props: { data: Awaited<ReturnType<typeof getVideo>> }) {
  const [favorite, setFavorite] = createSignal(false);

  onSettled(() => {
    setFavorite(isFavorite(props.data));
    recordViewed(props.data);
    setSeo({
      title: `${props.data.title} · PlantingTulips`,
      description: `Watch ${props.data.title} via the approved ${props.data.provider} embed and browse related catalog results.`,
      canonicalPath: `/watch/${props.data.provider}/${props.data.id}`,
      keywords: props.data.tags || [],
      image: props.data.thumbnailUrl,
      type: 'video.other',
    });
    setVideoStructuredData({
      name: props.data.title,
      description: `Embedded from ${props.data.provider}.`,
      thumbnailUrl: props.data.thumbnailUrl,
      embedUrl: props.data.embedUrl,
    });
    track('page_view', { provider: props.data.provider, videoId: props.data.id, metadata: { surface: 'watch' } });
  });

  const favoriteVideo = () => {
    const value = toggleFavorite(props.data);
    setFavorite(value);
    track(value ? 'favorite' : 'unfavorite', { provider: props.data.provider, videoId: props.data.id });
  };

  const hide = () => {
    hideVideo(props.data);
    track('hide', { provider: props.data.provider, videoId: props.data.id });
    window.location.href = '/';
  };

  return (
    <div>
      <iframe src={props.data.embedUrl} title={props.data.title} allowfullscreen allow="autoplay; fullscreen; picture-in-picture" referrerpolicy="strict-origin-when-cross-origin" {...stylex.props(s.player)} />
      <h1 {...stylex.props(s.playerTitle)}>{props.data.title}</h1>
      <div {...stylex.props(s.meta)}>{props.data.provider} · {props.data.duration}</div>
      <div {...stylex.props(s.tags)}><For each={props.data.tags || []}>{(tag) => <a href={`/tag/${slug(tag)}`} {...stylex.props(s.tag)}>{tag}</a>}</For></div>
      <div {...stylex.props(s.buttonRow)} style={{ 'margin-top': '18px' }}>
        <button type="button" onClick={favoriteVideo} {...stylex.props(favorite() ? s.primaryButton : s.secondaryButton)}>{favorite() ? '★ Favorited' : '☆ Favorite'}</button>
        <button type="button" onClick={hide} {...stylex.props(s.secondaryButton)}>Hide this video</button>
        <a href={props.data.sourceUrl} target="_blank" rel="noopener noreferrer" onClick={() => track('source_click', { provider: props.data.provider, videoId: props.data.id })} {...stylex.props(s.secondaryButton)}>Open source</a>
        <a href={`/report?provider=${props.data.provider}&id=${props.data.id}&url=${encodeURIComponent(props.data.sourceUrl)}`} {...stylex.props(s.secondaryButton)}>Report</a>
      </div>
    </div>
  );
}

function Related(props: { videos: Awaited<ReturnType<typeof getRelatedVideos>> }) {
  if (!props.videos.length) return null;
  return (
    <section style={{ 'margin-top': '28px' }}>
      <div {...stylex.props(s.contentHeader)}><div><h2 {...stylex.props(s.heading)}>More like this</h2><p {...stylex.props(s.subheading)}>Ranked by shared tags/categories, duration similarity, catalog quality, and anonymous engagement.</p></div></div>
      <div {...stylex.props(s.grid)}><For each={props.videos}>{(video) => <VideoCard video={video} context="related" />}</For></div>
    </section>
  );
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}
