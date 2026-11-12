import type { FoundDevice, Settings } from './types';

export class ApiError extends Error {}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  let json: { error?: string } = {};
  try {
    json = await res.json();
  } catch {
    /* empty body */
  }
  if (!res.ok) throw new ApiError(json.error ?? res.statusText);
  return json as T;
}

export const api = {
  saveSettings: (patch: Partial<Settings>) => request<unknown>('PUT', '/api/settings', patch),
  connect: () => request<unknown>('POST', '/api/connect'),
  disconnect: () => request<unknown>('POST', '/api/disconnect'),
  start: () => request<unknown>('POST', '/api/start'),
  stop: () => request<unknown>('POST', '/api/stop'),
  discover: () => request<FoundDevice[]>('GET', '/api/discover'),
};

export const frameUrl = (id: number) => `/api/frame.jpg?id=${id}`;
