import { For, Show, createSignal, onSettled } from 'solid-js';
import * as stylex from '@stylexjs/stylex';
import { blockAdminContent, getAdminReports } from '../lib/api';
import { setSeo } from '../lib/seo';
import type { AdminReport } from '../lib/types';
import { s } from '../ui/styles';

export function AdminReportsPage() {
  const [token, setToken] = createSignal('');
  const [reports, setReports] = createSignal<AdminReport[]>([]);
  const [status, setStatus] = createSignal('Enter the admin token to load reports.');

  onSettled(() => {
    setSeo({
      title: 'Moderation reports · PlantingTulips',
      description: 'Private PlantingTulips moderation report queue.',
      canonicalPath: '/admin/reports',
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
    setStatus('Loading reports…');
    try {
      const data = await getAdminReports(value);
      sessionStorage.setItem('pt-admin-token', value);
      setReports(data);
      setStatus(`${data.length} report${data.length === 1 ? '' : 's'} loaded.`);
    } catch (error) {
      setStatus(String(error));
    }
  }

  async function block(report: AdminReport) {
    if (!report.provider || !report.videoId) return;
    setStatus(`Blocking ${report.provider}/${report.videoId}…`);
    try {
      await blockAdminContent(token(), report);
      setStatus('Local catalog block applied. The item is now filtered independently of its provider.');
    } catch (error) {
      setStatus(String(error));
    }
  }

  return (
    <main {...stylex.props(s.page)}>
      <div {...stylex.props(s.panel)} style={{ 'margin-bottom': '18px' }}>
        <h1 {...stylex.props(s.heading)}>Moderation reports</h1>
        <p {...stylex.props(s.subheading)}>This operator-only view reads persisted reports from D1. Tokens are kept in session storage, not local storage.</p>
        <div {...stylex.props(s.buttonRow)}>
          <input type="password" value={token()} onInput={(event) => setToken(event.currentTarget.value)} placeholder="Admin token" {...stylex.props(s.input)} style={{ 'max-width': '420px' }} />
          <button type="button" onClick={() => void load()} {...stylex.props(s.primaryButton)}>Load reports</button>
        </div>
        <p {...stylex.props(s.subheading)}>{status()}</p>
      </div>
      <div style={{ display: 'grid', gap: '14px' }}>
        <For each={reports()}>{(report) => (
          <article {...stylex.props(s.panel)}>
            <div {...stylex.props(s.contentHeader)}>
              <div><strong>{report.reason}</strong><div {...stylex.props(s.subheading)}>{report.createdAt} · {report.email}</div></div>
              <span {...stylex.props(s.directoryBadge)}>{report.status}</span>
            </div>
            <p {...stylex.props(s.prose)}>{report.details}</p>
            <Show when={report.provider || report.videoId}><div {...stylex.props(s.meta)}>{report.provider || 'unknown'} / {report.videoId || 'unknown id'}</div></Show>
            <div {...stylex.props(s.buttonRow)} style={{ 'margin-top': '12px' }}>
              <Show when={report.sourceUrl}><a href={report.sourceUrl} target="_blank" rel="noopener noreferrer" {...stylex.props(s.secondaryButton)}>Open source</a></Show>
              <Show when={report.provider && report.videoId}><button type="button" onClick={() => void block(report)} {...stylex.props(s.primaryButton)}>Apply local block</button></Show>
            </div>
          </article>
        )}</For>
      </div>
    </main>
  );
}
