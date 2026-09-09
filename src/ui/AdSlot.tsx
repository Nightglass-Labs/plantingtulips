import { onSettled } from 'solid-js';
import { track } from '../lib/analytics';

type AdSlotName = 'feed-banner' | 'sidebar' | 'player-below' | 'native-card';

type AdSlotProps = {
  slot: AdSlotName;
};

export function AdSlot(props: AdSlotProps) {
  const mode = import.meta.env.VITE_AD_MODE || 'off';
  const zoneId = zoneFor(props.slot);

  onSettled(() => {
    if (mode !== 'exoclick' || !zoneId) return;
    const scriptUrl = import.meta.env.VITE_EXOCLICK_SCRIPT_URL || '';
    if (!validHttps(scriptUrl)) return;
    track('ad_impression', { slot: props.slot, metadata: { adProvider: 'exoclick' } });
    void serveExoClick(scriptUrl);
  });

  if (mode === 'off') return null;
  if (mode === 'preview') return <Preview slot={props.slot} />;
  if (mode !== 'exoclick' || !zoneId || !validHttps(import.meta.env.VITE_EXOCLICK_SCRIPT_URL || '')) return null;

  return (
    <aside aria-label="Advertisement" data-ad-slot={props.slot} style={{ margin: props.slot === 'sidebar' ? '0 0 16px' : '16px 0', 'text-align': 'center' }}>
      <ins class="adsbyexoclick" data-zoneid={zoneId} data-keywords={pageKeywords()} />
    </aside>
  );
}

function Preview(props: { slot: AdSlotName }) {
  return (
    <aside
      aria-label="Advertisement placeholder"
      data-ad-slot={props.slot}
      style={{
        border: '1px dashed #3b3b46',
        'border-radius': '10px',
        padding: '14px',
        margin: props.slot === 'sidebar' ? '0 0 16px' : '16px 0',
        'background-color': '#15151b',
        color: '#8f8f9b',
        'font-size': '12px',
        'text-align': 'center',
      }}
    >
      <strong style={{ color: '#d1d1d7' }}>Ad slot · {props.slot}</strong>
      <div>Preview mode. Production ad code is not loaded.</div>
    </aside>
  );
}

function zoneFor(slot: AdSlotName) {
  const map: Record<AdSlotName, string | undefined> = {
    'feed-banner': import.meta.env.VITE_EXOCLICK_ZONE_FEED_BANNER,
    sidebar: import.meta.env.VITE_EXOCLICK_ZONE_SIDEBAR,
    'player-below': import.meta.env.VITE_EXOCLICK_ZONE_PLAYER_BELOW,
    'native-card': import.meta.env.VITE_EXOCLICK_ZONE_NATIVE_CARD,
  };
  const value = map[slot];
  return value && /^\d+$/.test(value) ? value : '';
}

async function serveExoClick(scriptUrl: string) {
  await loadScript(scriptUrl);
  const target = window as typeof window & { AdProvider?: Array<Record<string, unknown>> };
  target.AdProvider = target.AdProvider || [];
  target.AdProvider.push({ serve: {} });
}

function loadScript(src: string) {
  const id = 'pt-exoclick-async';
  const existing = document.getElementById(id) as HTMLScriptElement | null;
  if (existing) return Promise.resolve();
  return new Promise<void>((resolve, reject) => {
    const script = document.createElement('script');
    script.id = id;
    script.async = true;
    script.type = 'application/javascript';
    script.src = src;
    script.setAttribute('data-cfasync', 'false');
    script.onload = () => resolve();
    script.onerror = () => reject(new Error('Ad provider script failed to load'));
    document.head.append(script);
  });
}

function pageKeywords() {
  return document.querySelector<HTMLMetaElement>('meta[name="keywords"]')?.content || '';
}

function validHttps(value: string) {
  try { return new URL(value).protocol === 'https:'; } catch { return false; }
}
