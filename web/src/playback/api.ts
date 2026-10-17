import { request } from "../api";
import type { Program } from "../scheduler/api";
export interface Channel {
  timezone?: string;
  id: string;
  number: string;
  name: string;
  logo: string;
  recentKind: string;
}
export interface OnNow {
  channel: Channel;
  program: Program | null;
  next: Program | null;
  progress: number;
}
export interface Capabilities {
  clientType: string;
  containers: string[];
  videoCodecs: string[];
  audioCodecs: string[];
  maxHeight: number;
  hls: boolean;
  forceTranscode?: boolean;
}
export interface Tune {
  id: string;
  channel: Channel;
  program: Program;
  next: Program | null;
  mode: "LIVE" | "BEHIND_LIVE" | "FROM_BEGINNING";
  state: string;
  serverTime: number;
  liveOffsetMs: number;
  positionMs: number;
  durationMs: number;
  streamUrl: string;
  mime: string;
  delivery: string;
  addedAt: number | null;
}
export interface Update {
  serverTime: number;
  mode: Tune["mode"];
  liveOffsetMs: number;
  transition: boolean;
  ended: boolean;
  message?: string;
}
export const playbackAPI = {
  onNow: () => request<{ items: OnNow[]; serverTime: number }>("/on-now"),
  tune: (
    channelId: string,
    mode: string,
    clientId: string,
    capabilities: Capabilities,
    csrf: string,
    replaceSession?: string,
  ) =>
    request<Tune>(
      "/playback/tune",
      "POST",
      { channelId, mode, clientId, capabilities, replaceSession },
      csrf,
    ),
  stop: (id: string, csrf: string) =>
    request(`/playback/${id}/stop`, "POST", {}, csrf),
  heartbeat: (
    id: string,
    positionMs: number,
    state: string,
    ended: boolean,
    csrf: string,
  ) =>
    request<Update>(
      `/playback/${id}/heartbeat`,
      "POST",
      { positionMs, state, ended },
      csrf,
    ),
  goLive: (id: string, csrf: string) =>
    request<Tune>(`/playback/${id}/go-live`, "POST", {}, csrf),
};
export const watch = (id: string) => {
  window.location.hash = `watch/${encodeURIComponent(id)}`;
};
