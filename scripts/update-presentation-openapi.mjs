import fs from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml");
const file = new URL("../openapi.yaml", import.meta.url);
const d = YAML.parse(fs.readFileSync(file, "utf8"));
d.info.version = "0.7.0";
d.info.description =
  "Self-hosted local inventory, deterministic broadcasts, authenticated Plex playback, guide/catch-up and personal viewer preferences. Mutations require exact Origin and session CSRF. Broadcast presentation is persisted in a unified timeline; optional clips consume real broadcast time.";
if (!d.tags.some((t) => t.name === "Presentation"))
  d.tags.push({ name: "Presentation" });
const s = d.components.schemas,
  str = { type: "string" },
  num = { type: "integer", format: "int64" },
  bool = { type: "boolean" };
const obj = (properties) => ({ type: "object", properties }),
  arr = (items) => ({ type: "array", items }),
  ref = (n) => ({ $ref: "#/components/schemas/" + n });
s.PresentationAsset = obj({
  id: str,
  name: str,
  kind: {
    type: "string",
    enum: [
      "BUMPER",
      "IDENT",
      "TRAILER",
      "COMING_UP_NEXT",
      "CUSTOM_CLIP",
      "FILLER",
    ],
  },
  source: {
    type: "string",
    enum: ["UPLOAD", "PLEX_MEDIA", "PLEX_EXTRA", "GENERATED_METADATA"],
  },
  mediaId: str,
  durationMs: { ...num, minimum: 1, maximum: 600000 },
  enabled: bool,
  available: bool,
  tags: arr(str),
  activeFrom: {
    type: "string",
    description:
      "Inclusive YYYY-MM-DD in channel timezone; empty means unbounded",
  },
  activeUntil: str,
  container: str,
  videoCodec: str,
  audioCodec: str,
  width: num,
  height: num,
  sizeBytes: num,
});
s.PresentationRule = obj({
  id: str,
  name: str,
  channelId: { type: "string", description: "Empty means global default" },
  enabled: bool,
  kind: s.PresentationAsset.properties.kind,
  tag: str,
  placement: {
    type: "string",
    enum: ["BEFORE", "BETWEEN", "HOURLY"],
    description:
      "HOURLY uses first safe boundary after an hour crossing, not a fixed clock slot",
  },
  mediaType: { type: "string", enum: ["", "movie", "episode"] },
  every: { type: "integer", minimum: 1 },
  repeatMinutes: num,
  maxDurationMs: num,
  priority: num,
});
s.ChannelPresentation = obj({
  channelId: str,
  enabled: bool,
  inherit: bool,
  bug: bool,
  corner: {
    type: "string",
    enum: ["top-left", "top-right", "bottom-left", "bottom-right"],
  },
  opacity: { type: "number", minimum: 0.1, maximum: 0.7 },
  size: { type: "integer", minimum: 32, maximum: 128 },
  compactLogoId: str,
  nextUpSeconds: { type: "integer", minimum: 0, maximum: 120 },
});
const id = { name: "id", in: "path", required: true, schema: str },
  security = [{ sessionCookie: [] }];
const op = (summary, schema, parameters = []) => ({
  tags: ["Presentation"],
  summary,
  security,
  parameters,
  responses: {
    200: {
      description: "Success",
      content: { "application/json": { schema } },
    },
    401: { description: "Sign in required" },
    403: { description: "Owner/admin role or Origin/CSRF required" },
    422: {
      description:
        "Invalid configuration, unsupported upload or referenced asset deletion",
    },
  },
});
const mutation = (summary, input, output, params = []) => ({
  ...op(summary, output, [
    ...params,
    { $ref: "#/components/parameters/Origin" },
    { $ref: "#/components/parameters/CSRF" },
  ]),
  requestBody: {
    required: true,
    content: { "application/json": { schema: input } },
  },
});
for (const [resource, schema] of [
  ["assets", "PresentationAsset"],
  ["rules", "PresentationRule"],
]) {
  const base = "/interstitials/" + resource;
  d.paths[base] = {
    get: op("Owner/admin presentation " + resource, arr(ref(schema))),
    post: mutation("Create " + resource, ref(schema), ref(schema)),
  };
  d.paths[base].post.responses[201] = d.paths[base].post.responses[200];
  delete d.paths[base].post.responses[200];
  d.paths[base + "/{id}"] = {
    patch: mutation(
      "Update " + resource + "; future changes applied by schedule repair",
      ref(schema),
      ref(schema),
      [id],
    ),
    delete: mutation(
      "Delete unreferenced " + resource,
      obj({}),
      obj({ deleted: bool }),
      [id],
    ),
  };
}
d.paths["/interstitials/rules"].post.responses[200] =
  d.paths["/interstitials/rules"].post.responses[201];
delete d.paths["/interstitials/rules"].post.responses[201];
d.paths["/interstitials/upload"] = {
  post: mutation(
    "Upload inspected MP4/H.264/AAC clip; 256 MiB default, 10 minutes and 1080p maximum",
    obj({}),
    ref("PresentationAsset"),
  ),
};
d.paths["/interstitials/upload"].post.requestBody.content = {
  "multipart/form-data": {
    schema: {
      type: "object",
      required: ["file", "name", "kind"],
      properties: {
        file: { type: "string", format: "binary" },
        name: str,
        kind: s.PresentationAsset.properties.kind,
      },
    },
  },
};
d.paths["/interstitials/upload"].post.responses[201] =
  d.paths["/interstitials/upload"].post.responses[200];
delete d.paths["/interstitials/upload"].post.responses[200];
d.paths["/interstitials/assets/{id}/preview"] = {
  get: {
    ...op("Authenticated admin local asset preview", str, [
      id,
      { name: "Range", in: "header", schema: str },
    ]),
    responses: {
      200: { description: "video/mp4" },
      206: { description: "Single byte range" },
      416: { description: "Unsatisfiable range" },
      401: { description: "Sign in required" },
      403: { description: "Admin only" },
      404: { description: "Local asset unavailable" },
    },
  },
};
d.paths["/interstitials/trailers"] = {
  post: mutation(
    "Discover real Plex trailer extras for a local media identity; no external scraping",
    obj({ mediaId: str }),
    obj({ added: num, available: num }),
  ),
};
d.paths["/channels/{id}/presentation"] = {
  get: op(
    "Read explicit or default channel branding",
    ref("ChannelPresentation"),
    [id],
  ),
  patch: mutation(
    "Save branding and insertion opt-in",
    ref("ChannelPresentation"),
    ref("ChannelPresentation"),
    [id],
  ),
};
d.paths["/channels/{id}/presentation/rules/{rule}"] = {
  put: mutation(
    "Enable/disable inherited rule for this channel",
    obj({ enabled: bool }),
    obj({ enabled: bool }),
    [id, { name: "rule", in: "path", required: true, schema: str }],
  ),
};
d.paths["/playback/{id}/advance"] = {
  post: mutation(
    "Viewer-owned live transition preserving viewer session; resolve authoritative timeline entry",
    obj({}),
    ref("PlaybackTune"),
    [
      id,
      {
        name: "fallback",
        in: "query",
        schema: bool,
        description:
          "Optional presentation failure uses metadata card for remainder of persisted interval",
      },
    ],
  ),
};
d.paths["/playback/{id}/advance"].post.tags = ["Playback"];
s.PlaybackTune.properties.branding = ref("ChannelPresentation");
Object.assign(s.ScheduledProgram.properties, {
  entryType: { type: "string", enum: ["PROGRAM", "INTERSTITIAL"] },
  assetId: str,
  followingProgramId: str,
});
if (s.PlaybackTune.properties.delivery)
  s.PlaybackTune.properties.delivery = {
    type: "string",
    enum: [
      "DIRECT_PLAY",
      "DIRECT_STREAM",
      "TRANSCODE",
      "PLEX_HLS",
      "LOCAL_INTERSTITIAL",
      "METADATA_PRESENTATION",
    ],
  };
for (const path of ["/guide/on-now", "/guide/starting-soon"])
  d.paths[path].get.description =
    "Clients must follow offset/limit while more is true. Mini-guide and Starting Soon aggregate all pages; no silent first-100 truncation.";
fs.writeFileSync(file, YAML.stringify(d, { lineWidth: 100 }));
