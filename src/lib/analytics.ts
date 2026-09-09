type EventName =
  | 'page_view'
  | 'search'
  | 'category_view'
  | 'video_impression'
  | 'video_open'
  | 'related_click'
  | 'source_click'
  | 'favorite'
  | 'unfavorite'
  | 'hide'
  | 'ad_impression'
  | 'ad_click';

type EventContext = {
  provider?: string;
  videoId?: string;
  category?: string;
  query?: string;
  slot?: string;
  value?: number;
  metadata?: Record<string, string | number | boolean | null | undefined>;
};

const eventsEndpoint = '/api/v1/events';

export function track(eventName: EventName, context: EventContext = {}) {
  if (typeof window === 'undefined') return;
  const body = JSON.stringify({
    sessionId: sessionId(),
    eventName,
    path: `${window.location.pathname}${window.location.search}`,
    ...context,
  });

  try {
    if (navigator.sendBeacon) {
      const sent = navigator.sendBeacon(eventsEndpoint, new Blob([body], { type: 'application/json' }));
      if (sent) return;
    }
    void fetch(eventsEndpoint, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body,
      keepalive: true,
    });
  } catch {
    // Analytics must never interrupt browsing.
  }
}

function sessionId() {
  const key = 'pt-session-id';
  const existing = sessionStorage.getItem(key);
  if (existing) return existing;
  const value = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  sessionStorage.setItem(key, value);
  return value;
}
