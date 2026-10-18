import { request } from "../api";
export interface Asset {
  id: string;
  name: string;
  kind: string;
  source: string;
  mediaId: string;
  durationMs: number;
  enabled: boolean;
  available: boolean;
  tags: string[];
  activeFrom: string;
  activeUntil: string;
}
export interface Rule {
  id: string;
  name: string;
  channelId: string;
  enabled: boolean;
  kind: string;
  tag: string;
  placement: string;
  mediaType: string;
  every: number;
  repeatMinutes: number;
  maxDurationMs: number;
  priority: number;
}
export interface Branding {
  channelId: string;
  enabled: boolean;
  inherit: boolean;
  bug: boolean;
  corner: string;
  opacity: number;
  size: number;
  compactLogoId: string;
  nextUpSeconds: number;
}
export const kinds = [
  "BUMPER",
  "IDENT",
  "TRAILER",
  "COMING_UP_NEXT",
  "CUSTOM_CLIP",
  "FILLER",
];
export const presentationAPI = {
  assets: () => request<Asset[]>("/interstitials/assets"),
  rules: () => request<Rule[]>("/interstitials/rules"),
  asset: (a: Partial<Asset>, csrf: string) =>
    request<Asset>(
      "/interstitials/assets" + (a.id ? "/" + a.id : ""),
      a.id ? "PATCH" : "POST",
      a,
      csrf,
    ),
  rule: (r: Rule, csrf: string) =>
    request<Rule>(
      "/interstitials/rules" + (r.id ? "/" + r.id : ""),
      r.id ? "PATCH" : "POST",
      r,
      csrf,
    ),
  branding: (id: string) => request<Branding>(`/channels/${id}/presentation`),
  saveBranding: (b: Branding, csrf: string) =>
    request(`/channels/${b.channelId}/presentation`, "PATCH", b, csrf),
  remove: (kind: string, id: string, csrf: string) =>
    request(`/interstitials/${kind}/${id}`, "DELETE", {}, csrf),
};
export function upload(
  file: File,
  name: string,
  kind: string,
  csrf: string,
  progress: (n: number) => void,
): Promise<Asset> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/v1/interstitials/upload");
    xhr.setRequestHeader("X-CSRF-Token", csrf);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) progress(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onerror = () => reject(new Error("Upload interrupted"));
    xhr.onload = () => {
      try {
        const v = JSON.parse(xhr.responseText);
        if (xhr.status >= 400) reject(new Error(v.error || "Upload failed"));
        else resolve(v);
      } catch {
        reject(new Error("Upload failed"));
      }
    };
    const form = new FormData();
    form.append("file", file);
    form.append("name", name);
    form.append("kind", kind);
    xhr.send(form);
  });
}
