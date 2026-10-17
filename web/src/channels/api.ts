import { APIError, request } from "../api";
export interface RuleNode {
  type: "group" | "condition";
  operator: string;
  field?: string;
  value?: string | number | string[] | number[];
  children?: RuleNode[];
}
export interface Rule {
  version: number;
  root: RuleNode;
}
export interface Source {
  recent?: { kind: "MOVIES" | "TV"; days: number };
  type: "STATIC" | "SHOW" | "COLLECTION" | "RULE";
  static?: { mediaIds: string[] };
  show?: {
    showId: string;
    episodeScope: "ALL" | "SELECTED_SEASONS";
    seasonIds: string[];
    includeSpecials: boolean;
    excludeUnavailable: boolean;
  };
  collection?: { collectionId: string };
  rule?: Rule;
}
export interface Appearance {
  source: "AUTO" | "PLEX" | "UPLOAD";
  uploadId: string | null;
}
export interface Definition {
  number: string;
  name: string;
  shortName: string;
  description: string;
  groupId: string | null;
  enabled: boolean;
  visibility: "VISIBLE" | "HIDDEN";
  sortOrder: number;
  logo: Appearance;
  background: Appearance;
  artworkMediaId: string | null;
  source: Source;
}
export interface Channel extends Definition {
  id: string;
  createdAt: number;
  updatedAt: number;
  count: number;
  warnings: string[];
}
export interface Group {
  id: string;
  name: string;
  slug: string;
  description: string;
  sortOrder: number;
  system: boolean;
  count: number;
}
export interface Media {
  id: string;
  title: string;
  type: string;
  year: number | null;
  showTitle: string;
  season: number | null;
  episode: number | null;
  durationMs: number | null;
  library: string;
  available: boolean;
  eligible: boolean;
}
export interface Preview {
  count: number;
  items: Media[];
  offset: number;
  limit: number;
  warnings: string[];
}
export interface Option {
  id: string;
  name: string;
  context: string;
  count: number;
  artworkId: string;
  seasons: number;
  episodes: number;
}
export interface Field {
  name: string;
  label: string;
  kind: string;
  operators: string[];
}
export const condition = (
  field = "media_type",
  operator = "is",
  value: RuleNode["value"] = "movie",
): RuleNode => ({ type: "condition", field, operator, value });
export const allRule = (): Rule => ({
  version: 1,
  root: condition("media_type", "in", ["movie", "episode"]),
});
export const newSource = (type: Source["type"]): Source => {
  switch (type) {
    case "STATIC":
      return { type, static: { mediaIds: [] } };
    case "SHOW":
      return {
        type,
        show: {
          showId: "",
          episodeScope: "ALL",
          seasonIds: [],
          includeSpecials: false,
          excludeUnavailable: true,
        },
      };
    case "COLLECTION":
      return { type, collection: { collectionId: "" } };
    case "RULE":
      return {
        type,
        rule: {
          version: 1,
          root: { type: "group", operator: "AND", children: [condition()] },
        },
      };
  }
};
export const newDefinition = (number: string): Definition => ({
  number,
  name: "",
  shortName: "",
  description: "",
  groupId: null,
  enabled: true,
  visibility: "VISIBLE",
  sortOrder: 0,
  logo: { source: "AUTO", uploadId: null },
  background: { source: "AUTO", uploadId: null },
  artworkMediaId: null,
  source: newSource("STATIC"),
});
export function definitionOf(c: Definition): Definition {
  return {
    number: c.number,
    name: c.name,
    shortName: c.shortName,
    description: c.description,
    groupId: c.groupId,
    enabled: c.enabled,
    visibility: c.visibility,
    sortOrder: c.sortOrder,
    logo: c.logo,
    background: c.background,
    artworkMediaId: c.artworkMediaId,
    source: c.source,
  };
}
export const channelAPI = {
  list: (offset = 0, group = "") =>
    request<{
      items: Channel[];
      more: boolean;
      nextNumber: string;
      recentKinds?: string[];
    }>(
      `/channels?offset=${offset}&limit=24&group=${encodeURIComponent(group)}`,
    ),
  get: (id: string) => request<Channel>(`/channels/${id}`),
  groups: () => request<Group[]>("/channel-groups"),
  fields: () => request<Field[]>("/channel-rules/fields"),
  save: (id: string | undefined, d: Definition, csrf: string) =>
    request<Channel>(
      `/channels${id ? `/${id}` : ""}`,
      id ? "PATCH" : "POST",
      d,
      csrf,
    ),
  remove: (id: string, csrf: string) =>
    request(`/channels/${id}`, "DELETE", undefined, csrf),
  duplicate: (id: string, csrf: string) =>
    request<Channel>(`/channels/${id}/duplicate`, "POST", undefined, csrf),
  enable: (id: string, enabled: boolean, csrf: string) =>
    request(`/channels/${id}/enabled`, "PATCH", { enabled }, csrf),
  preview: (
    source: Source,
    csrf: string,
    offset = 0,
    includeUnavailable = false,
  ) =>
    request<Preview>(
      "/channel-preview",
      "POST",
      { source, offset, limit: 24, includeUnavailable },
      csrf,
    ),
  metadata: (field: string, q = "", offset = 0, parent = "") =>
    request<{ items: Option[]; more: boolean }>(
      `/channel-metadata/${field}?${new URLSearchParams({ q, offset: String(offset), limit: "20", parent })}`,
    ),
  lookup: (field: string, value: string) =>
    request<Option>(
      `/channel-metadata/selected?${new URLSearchParams({ field, value })}`,
    ),
  saveGroup: (g: Partial<Group>, csrf: string) =>
    request<Group>(
      `/channel-groups${g.id ? `/${g.id}` : ""}`,
      g.id ? "PATCH" : "POST",
      g,
      csrf,
    ),
  removeGroup: (
    id: string,
    moveTo: string | null,
    ungroup: boolean,
    csrf: string,
  ) => request(`/channel-groups/${id}`, "DELETE", { moveTo, ungroup }, csrf),
  upload: async (file: File, csrf: string) => {
    const r = await fetch("/api/v1/channel-artwork", {
      method: "POST",
      credentials: "same-origin",
      headers: { "X-CSRF-Token": csrf, "Content-Type": file.type },
      body: file,
    });
    const d = await r.json();
    if (!r.ok)
      throw new APIError(r.status, d.error ?? "Upload failed", d.issues ?? []);
    return d as {
      id: string;
      contentType: string;
      width: number;
      height: number;
    };
  },
};
export const errorText = (e: unknown) =>
  e instanceof Error ? e.message : "Unable to complete channel operation";
export const duration = (ms: number | null) =>
  ms === null ? "Runtime unknown" : `${Math.round(ms / 60000)} min`;
export function moveItem(ids: string[], from: number, to: number) {
  const next = [...ids];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}
