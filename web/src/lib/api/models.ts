// HuggingFace model boards.

import { apiGet } from './client';

export type ModelTask =
  | 'text-gen' | 'multimodal' | 'image-gen' | 'video' | 'audio'
  | 'embedding' | 'vision' | 'nlp' | 'rl' | 'other';

export type ModelSort = 'hot' | 'downloads' | 'likes' | 'new';

export interface ModelItem {
  externalId: string; // author/model
  author?: string;
  name: string;
  task?: ModelTask;
  pipelineTag?: string;
  library?: string;
  license?: string;
  likes: number;
  downloads30d: number;
  downloadsAll?: number;
  params?: number; // parameter count
  growth: number; // daily likes gain
  gated: boolean;
  quantFormats?: string[];
  createdAt?: string | null;
  url: string;
}

/** One page of a model board. Empty on error so the page degrades. */
export async function getModels(opts: {
  task?: string;
  sort?: ModelSort;
  limit?: number;
  page?: number;
}): Promise<{ items: ModelItem[]; total: number }> {
  const params = new URLSearchParams();
  if (opts.task) params.set('task', opts.task);
  params.set('sort', opts.sort ?? 'hot');
  params.set('limit', String(opts.limit ?? 30));
  params.set('page', String(opts.page ?? 1));
  const res = await apiGet<{ data: ModelItem[]; total: number }>(`/models?${params}`, 1800);
  if (res.error !== null || !Array.isArray(res.data?.data)) return { items: [], total: 0 };
  return { items: res.data.data, total: res.data.total ?? res.data.data.length };
}
