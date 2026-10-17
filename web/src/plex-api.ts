import { request } from "./api";
export interface PlexServer {
  id: string;
  name: string;
  baseUrl: string;
  machineIdentifier: string;
  friendlyName: string;
  version: string;
  platform: string;
  status: string;
  lastSuccessAt: number | null;
  lastHealthCheckAt: number | null;
  lastError: string;
  disabled: boolean;
}
export interface PlexLibrary {
  id: string;
  serverId: string;
  key: string;
  title: string;
  type: string;
  selected: boolean;
  present: boolean;
  supported: boolean;
  lastSyncedAt: number | null;
  itemCount: number;
}
export interface PlexJob {
  id: string;
  type: string;
  serverId: string | null;
  libraryId: string | null;
  mode: string;
  status: string;
  processed: number;
  total: number | null;
  currentItem: string;
  error: string;
  createdAt: number;
  completedAt: number | null;
  cancelRequested: boolean;
}
export interface MediaItem {
  id: string;
  serverId: string;
  libraryId: string;
  libraryTitle: string;
  ratingKey: string;
  parentRatingKey: string;
  grandparentRatingKey: string;
  type: string;
  title: string;
  year: number | null;
  summary: string;
  available: boolean;
  updatedAt: number;
  index: number | null;
  parentIndex: number | null;
  contentRating: string;
  rating: number | null;
  tags?: { kind: string; relationship: string; name: string }[];
  externalIds?: { provider: string; value: string }[];
  versions?: {
    container: string;
    resolution: string;
    videoCodec: string;
    audioCodec: string;
    streams: { codec: string; language: string; type: number }[];
  }[];
}
export interface MediaResult {
  items: MediaItem[];
  total: number;
  offset: number;
  limit: number;
}
export interface PlexOptions {
  healthIntervalSeconds: number;
  syncIntervalSeconds: number;
  fullSyncIntervalSeconds: number;
  artworkCacheMb: number;
}
export interface Connection {
  baseUrl: string;
  token: string;
  name: string;
}
export const plexAPI = {
  servers: () => request<PlexServer[]>("/plex/servers"),
  libraries: (server: string) =>
    request<PlexLibrary[]>(`/plex/servers/${server}/libraries`),
  jobs: () => request<PlexJob[]>("/jobs"),
  settings: () => request<PlexOptions>("/plex/settings"),
  test: (value: Connection, csrf: string) =>
    request<{ friendlyName: string }>(
      "/plex/connections/test",
      "POST",
      value,
      csrf,
    ),
  save: (value: Connection, id: string | undefined, csrf: string) =>
    request<PlexServer>(
      `/plex/servers${id ? `/${id}` : ""}`,
      id ? "PATCH" : "POST",
      value,
      csrf,
    ),
  action: (id: string, action: "test" | "discover", csrf: string) =>
    request<PlexJob>(`/plex/servers/${id}/${action}`, "POST", undefined, csrf),
  remove: (id: string, csrf: string) =>
    request(`/plex/servers/${id}`, "DELETE", undefined, csrf),
  select: (id: string, selected: boolean, csrf: string) =>
    request(`/plex/libraries/${id}`, "PATCH", { selected }, csrf),
  sync: (id: string, mode: "incremental" | "full", csrf: string) =>
    request<PlexJob>(`/plex/libraries/${id}/sync`, "POST", { mode }, csrf),
  cancel: (id: string, csrf: string) =>
    request(`/jobs/${id}/cancel`, "POST", undefined, csrf),
  media: (params: URLSearchParams) => request<MediaResult>(`/media?${params}`),
  item: (id: string) => request<MediaItem>(`/media/${id}`),
  saveSettings: (value: PlexOptions, csrf: string) =>
    request<PlexOptions>("/plex/settings", "PUT", value, csrf),
};
export async function loadPlexOverview() {
  const [servers, jobs] = await Promise.all([
    plexAPI.servers(),
    plexAPI.jobs(),
  ]);
  const libraries = (
    await Promise.all(servers.map((server) => plexAPI.libraries(server.id)))
  ).flat();
  return { servers, jobs, libraries };
}
export const displayTime = (value: number | null) =>
  value ? new Date(value * 1000).toLocaleString() : "Not yet";
export const isActive = (job: PlexJob) =>
  job.status === "RUNNING" || job.status === "QUEUED";
