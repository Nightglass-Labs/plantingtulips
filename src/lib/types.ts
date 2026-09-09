export type ProviderName = 'eporner' | 'xvideos';

export type VideoSummary = {
  id: string;
  provider: ProviderName;
  title: string;
  thumbnailUrl: string;
  duration: string;
  views?: number;
  rating?: number;
  sourceUrl: string;
  embedUrl: string;
  tags?: string[];
};

export type VideoPageResult = {
  query: string;
  page: number;
  totalPages: number;
  totalCount: number;
  videos: VideoSummary[];
  source?: 'catalog' | 'live';
  providers: Record<ProviderName, { enabled: boolean; note?: string }>;
};

export type CategorySummary = {
  slug: string;
  name: string;
  description: string;
  query: string;
  videoCount: number;
};

export type AdminReport = {
  id: string;
  email: string;
  reason: string;
  provider?: string;
  videoId?: string;
  sourceUrl?: string;
  details: string;
  status: string;
  createdAt: string;
};

export type AdminDashboard = {
  catalog: { total: number; active: number };
  reports: Array<{ status: string; count: number }>;
  eventsToday: Array<{ event_name: string; count: number }>;
  providers: Array<{ name: ProviderName; enabled: number; lastOkAt?: string; lastErrorAt?: string; lastError?: string }>;
  sync: Array<{ provider: ProviderName; lastSyncAt?: string; lastRemovedSyncAt?: string; lastError?: string; importedCount: number }>;
  topSearches: Array<{ query: string; count: number }>;
  topCategories: Array<{ category: string; count: number }>;
  topVideos: Array<{ provider: ProviderName; videoId: string; opens: number; relatedClicks: number; sourceClicks: number; favorites: number; hides: number }>;
  ads: Array<{ day: string; slot: string; provider: string; impressions: number; clicks: number; revenue: number }>;
  funnel7d: {
    sessions: number;
    searches: number;
    feedImpressions: number;
    relatedImpressions: number;
    videoOpens: number;
    relatedClicks: number;
    sourceClicks: number;
    favorites: number;
  };
  analyticsRetention: {
    rawEvents: number;
    oldestEvent?: string | null;
    newestEvent?: string | null;
    configuredDays: number;
  };
};
