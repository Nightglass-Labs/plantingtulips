import type { AdminDashboard, AdminReport, CategorySummary, ProviderName, VideoPageResult, VideoSummary } from './types';

const api = '/api/v1';

export async function searchVideos(query: string, page = 1, sort = 'top-weekly'): Promise<VideoPageResult> {
  const url = new URL(`${api}/videos`, window.location.origin);
  url.searchParams.set('q', query || 'blowjob');
  url.searchParams.set('page', String(page));
  url.searchParams.set('sort', sort);
  const response = await fetch(url);
  if (!response.ok) throw new Error(`Catalog request failed (${response.status})`);
  return response.json();
}

export async function getVideo(provider: string, id: string): Promise<VideoSummary> {
  const response = await fetch(`${api}/video/${encodeURIComponent(provider)}/${encodeURIComponent(id)}`);
  if (!response.ok) throw new Error(`Video request failed (${response.status})`);
  return response.json();
}

export async function getRelatedVideos(provider: string, id: string): Promise<VideoSummary[]> {
  const response = await fetch(`${api}/related/${encodeURIComponent(provider)}/${encodeURIComponent(id)}`);
  if (!response.ok) return [];
  const data = await response.json() as { videos?: VideoSummary[] };
  return data.videos || [];
}

export async function getCategories(): Promise<{ categories: CategorySummary[]; configured: boolean }> {
  const response = await fetch(`${api}/categories`);
  if (!response.ok) return { categories: [], configured: false };
  return response.json();
}

export async function getAdminReports(token: string): Promise<AdminReport[]> {
  const response = await fetch(`${api}/admin/reports`, { headers: auth(token) });
  if (!response.ok) throw new Error(`Admin request failed (${response.status})`);
  const data = await response.json() as { reports?: AdminReport[] };
  return data.reports || [];
}

export async function getAdminDashboard(token: string): Promise<AdminDashboard> {
  const response = await fetch(`${api}/admin/dashboard`, { headers: auth(token) });
  if (!response.ok) throw new Error(`Dashboard request failed (${response.status})`);
  return response.json();
}

export async function blockAdminContent(token: string, report: AdminReport) {
  const response = await fetch(`${api}/admin/block`, {
    method: 'POST',
    headers: { ...auth(token), 'content-type': 'application/json' },
    body: JSON.stringify({ provider: report.provider, videoId: report.videoId, reason: `manual moderation block from report ${report.id}`, reportId: report.id }),
  });
  if (!response.ok) throw new Error(`Block request failed (${response.status})`);
}

export async function setProviderEnabled(token: string, provider: ProviderName, enabled: boolean) {
  const response = await fetch(`${api}/admin/provider`, {
    method: 'POST',
    headers: { ...auth(token), 'content-type': 'application/json' },
    body: JSON.stringify({ provider, enabled }),
  });
  if (!response.ok) throw new Error(`Provider update failed (${response.status})`);
}

export async function runCatalogSync(token: string, pages = 1) {
  const response = await fetch(`${api}/admin/ingest`, {
    method: 'POST',
    headers: { ...auth(token), 'content-type': 'application/json' },
    body: JSON.stringify({ pages, syncRemoved: true }),
  });
  if (!response.ok) throw new Error(`Catalog sync failed (${response.status})`);
  return response.json() as Promise<Record<string, unknown>>;
}

export async function runMaintenance(token: string, retentionDays = 90) {
  const response = await fetch(`${api}/admin/maintenance`, {
    method: 'POST',
    headers: { ...auth(token), 'content-type': 'application/json' },
    body: JSON.stringify({ retentionDays }),
  });
  if (!response.ok) throw new Error(`Maintenance failed (${response.status})`);
  return response.json() as Promise<{ ok: boolean; analytics: { retentionDays: number; deletedEvents: number; remainingEvents: number } }>;
}

function auth(token: string) {
  return { authorization: `Bearer ${token}` };
}
