import { request } from "../api";
export interface Program {
  id: string;
  channelId: string;
  mediaId: string;
  title: string;
  showTitle: string;
  type: string;
  start: number;
  end: number;
  durationMs: number;
  season: number | null;
  episode: number | null;
  available: boolean;
  artwork: string;
  state: "PAST" | "CURRENT" | "FUTURE";
  progress: number;
  canRestart: boolean;
}
export interface Channel {
  id: string;
  number: string;
  name: string;
  logo: string;
  groupId: string;
  group: string;
  favorite: boolean;
  status: string;
  recentKind: string;
  programs: Program[];
}
export interface WindowData {
  channels: Channel[];
  groups: { id: string; name: string }[];
  serverTime: number;
  timezone: string;
  more: boolean;
  start: number;
  end: number;
  warnings: string[];
}
export interface Detail extends Program {
  channel: Channel;
  summary: string;
  year: number | null;
  rating: number | null;
  contentRating: string;
  addedAt: number | null;
  genres: string[];
  background: string;
  serverTime: number;
  timezone: string;
}
export const guideAPI = {
  window: (
    start: number,
    end: number,
    group = "",
    favorites = false,
    offset = 0,
  ) =>
    request<WindowData>(
      "/guide?" +
        new URLSearchParams({
          start: new Date(start).toISOString(),
          end: new Date(end).toISOString(),
          group,
          favorites: String(favorites),
          offset: String(offset),
          limit: "50",
        }),
    ),
  now: () => allChannels("on-now"),
  soon: () => allChannels("starting-soon"),
  detail: (id: string) =>
    request<Detail>("/guide/programs/" + encodeURIComponent(id)),
  favorites: () => request<{ channelIds: string[] }>("/users/me/favorites"),
  favorite: (id: string, on: boolean, csrf: string) =>
    request(
      "/users/me/favorites/" + encodeURIComponent(id),
      on ? "POST" : "DELETE",
      {},
      csrf,
    ),
  last: () => request<{ channelId: string }>("/users/me/last-channel"),
};
export type TuneAction = (channel: string, program?: string) => void;

async function allChannels(kind: string): Promise<WindowData> {
  const first = await request<WindowData>(`/guide/${kind}?limit=100&offset=0`);
  let page = first;
  let offset = page.channels.length;
  while (page.more) {
    if (!page.channels.length)
      throw new Error("Channel pagination did not advance");
    page = await request<WindowData>(
      `/guide/${kind}?limit=100&offset=${offset}`,
    );
    first.channels.push(...page.channels);
    first.warnings.push(...page.warnings);
    offset += page.channels.length;
  }
  return { ...first, more: false };
}
