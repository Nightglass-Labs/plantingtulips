import * as stylex from '@stylexjs/stylex';
import type { VideoSummary } from '../lib/types';
import { track } from '../lib/analytics';
import { isHidden } from '../lib/personalization';
import { s } from './styles';

export function VideoCard(props: { video: VideoSummary; context?: 'feed' | 'related' | 'library' }) {
  if (isHidden(props.video)) return null;

  track('video_impression', {
    provider: props.video.provider,
    videoId: props.video.id,
    metadata: { context: props.context || 'feed' },
  });

  const open = () => {
    track(props.context === 'related' ? 'related_click' : 'video_open', {
      provider: props.video.provider,
      videoId: props.video.id,
      metadata: { context: props.context || 'feed' },
    });
  };

  return (
    <a href={`/watch/${props.video.provider}/${props.video.id}`} onClick={open} {...stylex.props(s.card)}>
      <div {...stylex.props(s.thumbWrap)}>
        <img src={props.video.thumbnailUrl} alt="" loading="lazy" {...stylex.props(s.thumb)} />
        <span {...stylex.props(s.provider)}>{props.video.provider}</span>
        <span {...stylex.props(s.duration)}>{props.video.duration}</span>
      </div>
      <div {...stylex.props(s.cardTitle)}>{props.video.title}</div>
      <div {...stylex.props(s.meta)}>{formatViews(props.video.views)}{props.video.rating ? ` · ${props.video.rating.toFixed(1)}%` : ''}</div>
    </a>
  );
}

function formatViews(value?: number) {
  if (!value) return '';
  return `${new Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 1 }).format(value)} views`;
}
