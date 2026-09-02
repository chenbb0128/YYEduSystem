import { businessRequestClient } from '#/api/request';

export type DailySummaryStatus = 'closed' | 'draft' | 'published' | 'withdrawn';

export interface DailySummaryRecord {
  id: number;
  summary_date: string;
  content: string;
  child_updates?: Record<string, string>;
  status: DailySummaryStatus;
  version: number;
  created_by_name: string;
  generated_at?: string;
  published_at?: string;
  closed_at?: string;
  withdrawn_at?: string;
  withdrawal_reason?: string;
  correction_reason?: string;
  created_at: string;
  updated_at: string;
}

export interface DailySummaryVersionRecord {
  id: number;
  version: number;
  action: string;
  content: string;
  child_updates?: Record<string, string>;
  reason?: string;
  created_by_name: string;
  created_at: string;
}

interface PageResult<T> {
  items: T[];
  total: number;
}

export function getDailySummariesApi(params?: { date?: string }) {
  return businessRequestClient.get<PageResult<DailySummaryRecord>>(
    '/daily-summaries',
    { params },
  );
}

export function generateDailySummaryApi(summaryDate: string) {
  return businessRequestClient.post<DailySummaryRecord>(
    '/daily-summaries/generate',
    { summary_date: summaryDate },
  );
}

export function getDailySummaryVersionsApi(id: number) {
  return businessRequestClient.get<{
    items: DailySummaryVersionRecord[];
    total: number;
  }>(`/daily-summaries/${id}/versions`);
}

export function updateDailySummaryApi(
  id: number,
  data: { child_updates?: Record<string, string>; content: string },
) {
  return businessRequestClient.put<DailySummaryRecord>(
    `/daily-summaries/${id}`,
    data,
  );
}

export function publishDailySummaryApi(id: number) {
  return businessRequestClient.post<DailySummaryRecord>(
    `/daily-summaries/${id}/publish`,
  );
}

export function closeDailySummaryApi(id: number) {
  return businessRequestClient.post<DailySummaryRecord>(
    `/daily-summaries/${id}/close`,
  );
}

export function withdrawDailySummaryApi(id: number, reason: string) {
  return businessRequestClient.post<DailySummaryRecord>(
    `/daily-summaries/${id}/withdraw`,
    { reason },
  );
}

export function correctDailySummaryApi(
  id: number,
  data: {
    child_updates?: Record<string, string>;
    content: string;
    reason: string;
  },
) {
  return businessRequestClient.post<DailySummaryRecord>(
    `/daily-summaries/${id}/correct`,
    data,
  );
}
