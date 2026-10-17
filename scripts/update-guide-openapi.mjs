import fs from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml");
const file = new URL("../openapi.yaml", import.meta.url),
  d = YAML.parse(fs.readFileSync(file, "utf8"));
d.info.version = "0.6.0";
d.info.description =
  "Self-hosted local inventory, deterministic broadcasts, authenticated Plex playback, guide/catch-up and personal viewer preferences. Mutations require exact Origin and session CSRF. No DVR recording or external IPTV.";
if (!d.tags.some((t) => t.name === "Guide")) d.tags.push({ name: "Guide" });
const s = d.components.schemas,
  r = (n) => ({ $ref: "#/components/schemas/" + n }),
  str = { type: "string" },
  num = { type: "integer", format: "int64" },
  bool = { type: "boolean" },
  arr = (items) => ({ type: "array", items }),
  obj = (properties) => ({ type: "object", properties });
s.GuideProgram = obj({
  id: str,
  channelId: str,
  mediaId: str,
  title: str,
  showTitle: str,
  type: str,
  start: num,
  end: num,
  durationMs: num,
  season: { ...num, nullable: true },
  episode: { ...num, nullable: true },
  available: bool,
  artwork: str,
  state: { type: "string", enum: ["PAST", "CURRENT", "FUTURE"] },
  progress: { type: "number" },
  canRestart: bool,
});
s.GuideChannel = obj({
  id: str,
  number: str,
  name: str,
  logo: str,
  groupId: str,
  group: str,
  favorite: bool,
  status: str,
  recentKind: str,
  programs: arr(r("GuideProgram")),
});
s.GuideWindow = obj({
  channels: arr(r("GuideChannel")),
  groups: arr(obj({ id: str, name: str })),
  serverTime: num,
  timezone: str,
  start: num,
  end: num,
  more: bool,
  warnings: arr(str),
});
s.GuideDetail = {
  allOf: [
    r("GuideProgram"),
    obj({
      channel: r("GuideChannel"),
      summary: str,
      year: { ...num, nullable: true },
      rating: { type: "number", nullable: true },
      contentRating: str,
      addedAt: { ...num, nullable: true },
      genres: arr(str),
      background: str,
      serverTime: num,
      timezone: str,
    }),
  ],
};
const op = (summary, schema, parameters = []) => ({
  tags: ["Guide"],
  summary,
  security: [{ sessionCookie: [] }],
  parameters,
  responses: {
    200: {
      description: "Success",
      content: { "application/json": { schema } },
    },
    400: { description: "Invalid or too-dense window" },
    401: { description: "Sign in required" },
    404: { description: "Disabled/hidden/missing channel or expired program" },
  },
});
const filters = [
  { name: "group", in: "query", schema: str },
  { name: "favorites", in: "query", schema: bool },
  { name: "offset", in: "query", schema: { type: "integer", minimum: 0 } },
  {
    name: "limit",
    in: "query",
    schema: { type: "integer", minimum: 1, maximum: 100, default: 50 },
  },
];
d.paths["/guide"] = {
  get: op(
    "Local broadcast window, maximum 24 hours and 10000 programs; channel pagination; empty channels excluded",
    r("GuideWindow"),
    [
      {
        name: "start",
        in: "query",
        required: true,
        schema: { type: "string", format: "date-time" },
      },
      {
        name: "end",
        in: "query",
        required: true,
        schema: { type: "string", format: "date-time" },
      },
      ...filters,
    ],
  ),
};
for (const [path, text] of [
  ["on-now", "Local current/next by channel"],
  [
    "starting-soon",
    "Programs starting in the next 30 minutes, grouped by channel; sort flattened programs by start for display",
  ],
])
  d.paths["/guide/" + path] = { get: op(text, r("GuideWindow"), filters) };
const id = { name: "id", in: "path", required: true, schema: str };
d.paths["/guide/programs/{id}"] = {
  get: op(
    "Retained program details; catch-up requires start <= server time and available media",
    r("GuideDetail"),
    [id],
  ),
};
d.paths["/users/me/favorites"] = {
  get: op(
    "Current user favorite enabled channels, normal channel ordering",
    obj({ channelIds: arr(str) }),
  ),
};
d.paths["/users/me/favorites/{id}"] = {};
for (const method of ["post", "delete"])
  d.paths["/users/me/favorites/{id}"][method] = {
    ...op(
      method === "post" ? "Favorite own channel" : "Unfavorite own channel",
      obj({ favorite: bool }),
      [
        id,
        { $ref: "#/components/parameters/Origin" },
        { $ref: "#/components/parameters/CSRF" },
      ],
    ),
    requestBody: {
      required: false,
      content: { "application/json": { schema: obj({}) } },
    },
  };
d.paths["/users/me/last-channel"] = {
  get: op(
    "Last successfully tuned enabled/visible channel for current user",
    obj({ channelId: str }),
  ),
};
s.PlaybackTune.properties.origin = {
  type: "string",
  enum: ["LIVE", "CURRENT_FROM_BEGINNING", "CATCH_UP"],
};
d.paths["/playback/tune"].post.requestBody.content[
  "application/json"
].schema.properties.programId = {
  type: "string",
  description:
    "Optional retained scheduler identity for FROM_BEGINNING. Must belong to channel and have already started. Historical playback is CATCH_UP.",
};
d.paths["/playback/tune"].post.responses[429] = {
  description: "Bounded tune/session capacity reached",
};
d.paths["/watch/channels/{id}/programs/{program}/artwork"].get.parameters =
  d.paths[
    "/watch/channels/{id}/programs/{program}/artwork"
  ].get.parameters.filter((p) => p.name !== "kind");
d.paths["/watch/channels/{id}/programs/{program}/artwork"].get.parameters.push({
  name: "kind",
  in: "query",
  schema: { type: "string", enum: ["poster", "background"], default: "poster" },
});
fs.writeFileSync(file, YAML.stringify(d, { lineWidth: 100 }));
