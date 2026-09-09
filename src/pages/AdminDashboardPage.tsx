import { For, Show, createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { getAdminDashboard, runCatalogSync, runMaintenance, setProviderEnabled } from '../lib/api';
import { setSeo } from '../lib/seo';
import type { AdminDashboard, ProviderName } from '../lib/types';
import { s } from '../ui/styles';

export function AdminDashboardPage() {
  const [token, setToken] = createSignal('');
  const [dashboard, setDashboard] = createSignal<AdminDashboard | null>(null);
  const [status, setStatus] = createSignal('Enter the admin token to load operations data.');
  const [syncing, setSyncing] = createSignal(false);
  const [maintaining, setMaintaining] = createSignal(false);

  onSettled(() => {
    setSeo({
      title: 'Admin · PlantingTulips',
      description: 'Private PlantingTulips operator dashboard.',
      canonicalPath: '/admin',
      robots: 'noindex,nofollow,noarchive',
    });
    const saved = sessionStorage.getItem('pt-admin-token') || '';
    if (saved) {
      setToken(saved);
      void load(saved);
    }
  });

  async function load(value = token()) {
    if (!value) return;
    setStatus('Loading dashboard…');
    try {
      const data = await getAdminDashboard(value);
      sessionStorage.setItem('pt-admin-token', value);
      setDashboard(data);
      setStatus('Dashboard loaded.');
    } catch (error) {
      setStatus(String(error));
    }
  }

  async function toggle(provider: ProviderName, enabled: boolean) {
    setStatus(`${enabled ? 'Enabling' : 'Disabling'} ${provider}…`);
    try {
      await setProviderEnabled(token(), provider, enabled);
      await load();
      setStatus(`${provider} ${enabled ? 'enabled' : 'disabled'}.`);
    } catch (error) {
      setStatus(String(error));
    }
  }

  async function sync() {
    setSyncing(true);
    setStatus('Running catalog sync…');
    try {
      const result = await runCatalogSync(token(), 1);
      setStatus(`Catalog sync complete: ${JSON.stringify(result)}`);
      await load();
    } catch (error) {
      setStatus(String(error));
    } finally {
      setSyncing(false);
    }
  }

  async function maintain() {
    setMaintaining(true);
    setStatus('Pruning raw analytics outside the retention window…');
    try {
      const result = await runMaintenance(token(), 90);
      setStatus(`Maintenance complete: ${result.analytics.deletedEvents.toLocaleString()} raw events deleted; ${result.analytics.remainingEvents.toLocaleString()} remain.`);
      await load();
    } catch (error) {
      setStatus(String(error));
    } finally {
      setMaintaining(false);
    }
  }

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.panel)} style={{ 'margin-bottom': '18px' }}>
        <div {...stylex.props(s.contentHeader)}>
          <div><h1 {...stylex.props(s.heading)}>Operations</h1><p {...stylex.props(s.subheading)}>Catalog, providers, discovery, moderation, privacy, and monetization in one operator view.</p></div>
          <a href="/admin/reports" {...stylex.props(s.secondaryButton)}>Reports</a>
        </div>
        <div {...stylex.props(s.buttonRow)}>
          <input type="password" value={token()} onInput={(event) => setToken(event.currentTarget.value)} placeholder="Admin token" {...stylex.props(s.input)} style={{ 'max-width': '420px' }} />
          <button type="button" onClick={() => void load()} {...stylex.props(s.primaryButton)}>Load</button>
          <button type="button" disabled={syncing()} onClick={() => void sync()} {...stylex.props(s.secondaryButton)}>{syncing() ? 'Syncing…' : 'Sync catalog'}</button>
          <button type="button" disabled={maintaining()} onClick={() => void maintain()} {...stylex.props(s.secondaryButton)}>{maintaining() ? 'Maintaining…' : 'Prune analytics'}</button>
        </div>
        <p {...stylex.props(s.subheading)} style={{ 'overflow-wrap': 'anywhere' }}>{status()}</p>
      </div>

      <Show when={dashboard()}>{(data) => (
        <>
          <div style={{ display: 'grid', 'grid-template-columns': 'repeat(auto-fit,minmax(180px,1fr))', gap: '12px', 'margin-bottom': '20px' }}>
            <Stat label="Catalog" value={`${data().catalog.active.toLocaleString()} active`} detail={`${data().catalog.total.toLocaleString()} total`} />
            <Stat label="Reports" value={String(data().reports.reduce((sum, row) => sum + Number(row.count || 0), 0))} detail="all statuses" />
            <Stat label="Events / 24h" value={String(data().eventsToday.reduce((sum, row) => sum + Number(row.count || 0), 0))} detail="anonymous events" />
            <Stat label="Ad impressions / 30d" value={String(data().ads.reduce((sum, row) => sum + Number(row.impressions || 0), 0))} detail="first-party count" />
          </div>

          <Section title="7-day funnel">
            <div style={{ display: 'grid', 'grid-template-columns': 'repeat(auto-fit,minmax(160px,1fr))', gap: '10px' }}>
              <Stat label="Sessions" value={data().funnel7d.sessions.toLocaleString()} detail="anonymous browser sessions" />
              <Stat label="Searches" value={data().funnel7d.searches.toLocaleString()} detail={rate(data().funnel7d.searches, data().funnel7d.sessions, 'per session')} />
              <Stat label="Discovery impressions" value={data().funnel7d.feedImpressions.toLocaleString()} detail="feed + library cards" />
              <Stat label="Video opens" value={data().funnel7d.videoOpens.toLocaleString()} detail={percent(data().funnel7d.videoOpens, data().funnel7d.feedImpressions, 'discovery CTR')} />
              <Stat label="Related impressions" value={data().funnel7d.relatedImpressions.toLocaleString()} detail="more-like-this cards" />
              <Stat label="Related clicks" value={data().funnel7d.relatedClicks.toLocaleString()} detail={percent(data().funnel7d.relatedClicks, data().funnel7d.relatedImpressions, 'related CTR')} />
              <Stat label="Source clicks" value={data().funnel7d.sourceClicks.toLocaleString()} detail={percent(data().funnel7d.sourceClicks, data().funnel7d.videoOpens, 'of opens')} />
              <Stat label="Favorites" value={data().funnel7d.favorites.toLocaleString()} detail={percent(data().funnel7d.favorites, data().funnel7d.videoOpens, 'of opens')} />
            </div>
          </Section>

          <Section title="Analytics retention">
            <div {...stylex.props(s.panel)}>
              <strong>{data().analyticsRetention.rawEvents.toLocaleString()} raw events retained</strong>
              <div {...stylex.props(s.subheading)}>{data().analyticsRetention.configuredDays}-day retention · oldest {String(data().analyticsRetention.oldestEvent || 'none')} · newest {String(data().analyticsRetention.newestEvent || 'none')}</div>
            </div>
          </Section>

          <Section title="Providers">
            <For each={data().providers}>{(provider) => (
              <div {...stylex.props(s.panel)}>
                <div {...stylex.props(s.contentHeader)}>
                  <div><strong>{provider.name}</strong><div {...stylex.props(s.subheading)}>{provider.lastError || provider.lastOkAt || 'No health check yet'}</div></div>
                  <button type="button" onClick={() => void toggle(provider.name, provider.enabled === 0)} {...stylex.props(provider.enabled ? s.secondaryButton : s.primaryButton)}>{provider.enabled ? 'Disable' : 'Enable'}</button>
                </div>
              </div>
            )}</For>
          </Section>

          <Section title="Top searches">
            <MetricRows rows={data().topSearches.map((row) => [row.query, row.count])} />
          </Section>

          <Section title="Top categories">
            <MetricRows rows={data().topCategories.map((row) => [row.category, row.count])} />
          </Section>

          <Section title="Top videos">
            <div style={{ display: 'grid', gap: '8px' }}><For each={data().topVideos}>{(row) => <div {...stylex.props(s.panel)}><strong>{row.provider}/{row.videoId}</strong><div {...stylex.props(s.subheading)}>opens {row.opens} · related {row.relatedClicks} · source {row.sourceClicks} · favorites {row.favorites} · hides {row.hides}</div></div>}</For></div>
          </Section>

          <Section title="Monetization">
            <div style={{ display: 'grid', gap: '8px' }}><For each={data().ads}>{(row) => <div {...stylex.props(s.panel)}><strong>{row.day} · {row.slot} · {row.provider}</strong><div {...stylex.props(s.subheading)}>{row.impressions} impressions · {row.clicks} clicks · ${Number(row.revenue || 0).toFixed(2)} recorded revenue</div></div>}</For></div>
          </Section>
        </>
      )}</Show>
    </main>
  );
}

function Stat(props: { label: string; value: string; detail: string }) {
  return <div {...stylex.props(s.panel)}><div {...stylex.props(s.subheading)}>{props.label}</div><div style={{ 'font-size': '22px', 'font-weight': '700', 'margin-top': '5px' }}>{props.value}</div><div {...stylex.props(s.meta)}>{props.detail}</div></div>;
}

function Section(props: { title: string; children: unknown }) {
  return <section style={{ 'margin-bottom': '24px' }}><h2 {...stylex.props(s.heading)}>{props.title}</h2>{props.children as any}</section>;
}

function MetricRows(props: { rows: Array<[string, number]> }) {
  return <div style={{ display: 'grid', gap: '8px' }}><For each={props.rows}>{([label, count]) => <div {...stylex.props(s.panel)} style={{ display: 'flex', 'justify-content': 'space-between', gap: '14px' }}><span>{label}</span><strong>{Number(count).toLocaleString()}</strong></div>}</For></div>;
}

function rate(numerator: number, denominator: number, suffix: string) {
  if (!denominator) return `0 ${suffix}`;
  return `${(numerator / denominator).toFixed(2)} ${suffix}`;
}

function percent(numerator: number, denominator: number, suffix: string) {
  if (!denominator) return `0% ${suffix}`;
  return `${((numerator / denominator) * 100).toFixed(1)}% ${suffix}`;
}
