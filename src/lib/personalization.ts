import type { VideoSummary } from './types';

const key = 'pt-personalization-v1';

type State = {
  favorites: VideoSummary[];
  recent: VideoSummary[];
  hidden: string[];
  tagWeights: Record<string, number>;
};

const empty = (): State => ({ favorites: [], recent: [], hidden: [], tagWeights: {} });

export function getPersonalization(): State {
  if (typeof localStorage === 'undefined') return empty();
  try {
    const parsed = JSON.parse(localStorage.getItem(key) || '') as Partial<State>;
    return {
      favorites: Array.isArray(parsed.favorites) ? parsed.favorites.slice(0, 200) : [],
      recent: Array.isArray(parsed.recent) ? parsed.recent.slice(0, 50) : [],
      hidden: Array.isArray(parsed.hidden) ? parsed.hidden.slice(0, 500) : [],
      tagWeights: parsed.tagWeights && typeof parsed.tagWeights === 'object' ? parsed.tagWeights : {},
    };
  } catch {
    return empty();
  }
}

export function recordViewed(video: VideoSummary) {
  const state = getPersonalization();
  state.recent = [video, ...state.recent.filter((item) => id(item) !== id(video))].slice(0, 50);
  for (const tag of video.tags || []) {
    const normalized = tag.toLowerCase().trim();
    if (normalized) state.tagWeights[normalized] = Math.min(100, (state.tagWeights[normalized] || 0) + 1);
  }
  save(state);
}

export function toggleFavorite(video: VideoSummary) {
  const state = getPersonalization();
  const exists = state.favorites.some((item) => id(item) === id(video));
  state.favorites = exists
    ? state.favorites.filter((item) => id(item) !== id(video))
    : [video, ...state.favorites].slice(0, 200);
  save(state);
  return !exists;
}

export function hideVideo(video: VideoSummary) {
  const state = getPersonalization();
  const value = id(video);
  if (!state.hidden.includes(value)) state.hidden = [value, ...state.hidden].slice(0, 500);
  state.favorites = state.favorites.filter((item) => id(item) !== value);
  save(state);
}

export function isFavorite(video: VideoSummary) {
  return getPersonalization().favorites.some((item) => id(item) === id(video));
}

export function isHidden(video: VideoSummary) {
  return getPersonalization().hidden.includes(id(video));
}

export function clearHistory() {
  const state = getPersonalization();
  state.recent = [];
  save(state);
}

export function preferredTags(limit = 8) {
  return Object.entries(getPersonalization().tagWeights)
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([tag]) => tag);
}

function save(state: State) {
  try { localStorage.setItem(key, JSON.stringify(state)); } catch { /* local personalization is best effort */ }
}

function id(video: Pick<VideoSummary, 'provider' | 'id'>) {
  return `${video.provider}:${video.id}`;
}
