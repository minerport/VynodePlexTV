import { request } from "../api";
export interface ScheduleSettings {
  recencyWeighted?: boolean;
  avoidSameShow?: boolean;
  minOtherShows?: number;
  channelId: string;
  mode: string;
  timezone: string;
  anchorMs: number;
  seed: string;
  movieRepeatDays: number;
  episodeRepeatDays: number;
  pastDays: number;
  futureDays: number;
  includeUnavailable: boolean;
  enabled: boolean;
}
export interface ScheduleStatus {
  channelId: string;
  name: string;
  number: string;
  mode: string;
  timezone: string;
  status: string;
  resolved: number;
  schedulable: number;
  excluded: number;
  coverageStart: number | null;
  coverageEnd: number | null;
  programCount: number;
  lastGenerated: number | null;
  lastError: string;
  integrity: string[];
  warnings: string[];
  revision: number;
}
export interface Program {
  id: string;
  mediaId: string;
  start: number;
  end: number;
  durationMs: number;
  title: string;
  showTitle: string;
  season: number | null;
  episode: number | null;
  available: boolean;
  artwork: string;
  policyReason: string;
}
export interface ProgramPage {
  items: Program[];
  more: boolean;
  timezone: string;
}
export interface ScheduleJob {
  id: string;
  type: string;
  status: string;
  processed: number;
  repaired: number;
  error: string;
  createdAt: number;
}
const base = (id: string) => `/channels/${encodeURIComponent(id)}/schedule`;
export const scheduleAPI = {
  list: (offset = 0) =>
    request<ScheduleStatus[]>(`/scheduler?offset=${offset}&limit=30`),
  settings: (id: string) => request<ScheduleSettings>(base(id) + "/settings"),
  status: (id: string) => request<ScheduleStatus>(base(id) + "/status"),
  programs: (id: string, day: string, offset = 0) =>
    request<ProgramPage>(`${base(id)}?day=${day}&offset=${offset}&limit=100`),
  jobs: (id: string) =>
    request<ScheduleJob[]>(`/scheduler/jobs?channel=${encodeURIComponent(id)}`),
  save: (id: string, v: ScheduleSettings, csrf: string) =>
    request<ScheduleSettings>(base(id) + "/settings", "PATCH", v, csrf),
  run: (id: string, action: string, csrf: string) =>
    request<{ jobId: string }>(base(id) + "/" + action, "POST", {}, csrf),
};
export function localDay(zone: string, at = new Date()) {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(at);
}
export function shiftDay(day: string, delta: number) {
  const d = new Date(day + "T12:00:00Z");
  d.setUTCDate(d.getUTCDate() + delta);
  return d.toISOString().slice(0, 10);
}
export function displayTime(at: number | null, zone: string) {
  try {
    return at === null
      ? "Not generated"
      : new Intl.DateTimeFormat(undefined, {
          timeZone: zone,
          month: "short",
          day: "numeric",
          hour: "numeric",
          minute: "2-digit",
          second: "2-digit",
          timeZoneName: "short",
        }).format(at);
  } catch {
    return "Enter a valid timezone";
  }
}
