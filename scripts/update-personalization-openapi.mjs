import fs from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml"),
  file = new URL("../openapi.yaml", import.meta.url),
  d = YAML.parse(fs.readFileSync(file, "utf8"));
d.info.version = "0.8.0";
if (!d.tags.some((t) => t.name === "Personalization"))
  d.tags.push({ name: "Personalization" });
const str = { type: "string" },
  num = { type: "integer", format: "int64" },
  bool = { type: "boolean" },
  obj = (properties) => ({ type: "object", properties }),
  arr = (items) => ({ type: "array", items }),
  ref = (n) => ({ $ref: "#/components/schemas/" + n }),
  id = { name: "id", in: "path", required: true, schema: str };
const s = d.components.schemas;
s.PersonalChannel = obj({
  id: str,
  number: str,
  name: str,
  hidden: bool,
  favorite: bool,
});
s.PersonalChannels = obj({
  mode: { type: "string", enum: ["NUMBER", "PERSONAL"] },
  channels: arr(ref("PersonalChannel")),
});
s.ViewingHistory = obj({
  id: str,
  title: str,
  channelId: { ...str, nullable: true },
  startedAt: num,
  endedAt: { ...num, nullable: true },
  watchDurationMs: num,
  programDurationMs: num,
  completionRatio: { type: "number", minimum: 0, maximum: 1 },
  origin: str,
});
s.ContentAffinity = obj({
  kind: {
    type: "string",
    enum: ["genre", "show", "actor", "director", "channel"],
  },
  identity: str,
  name: str,
  score: { type: "number", minimum: 0, maximum: 1 },
  engagedSessions: num,
});
s.Recommendation = obj({
  channel: ref("GuideChannel"),
  program: ref("GuideProgram"),
  reasons: arr(str),
  score: { type: "number", description: "Own-user debug only" },
  components: {
    type: "object",
    additionalProperties: { type: "number" },
    description: "Own-user debug only",
  },
});
s.Recommendations = obj({
  items: arr(ref("Recommendation")),
  rankingVersion: num,
  serverTime: num,
  coldStart: bool,
});
s.SmartTemplate = obj({ id: str, name: str, description: str, field: str });
s.SmartTemplateInput = obj({
  template: {
    type: "string",
    enum: [
      "genre",
      "actor",
      "director",
      "decade",
      "comfort",
      "weekend",
      "family",
      "seasonal",
    ],
  },
  selection: str,
  name: str,
  number: str,
  mediaType: { type: "string", enum: ["", "movie", "episode"] },
  decade: { type: "integer" },
  startDate: { type: "string", format: "date" },
  endDate: { type: "string", format: "date" },
  timezone: str,
});
s.ActivationWindow = obj({
  enabled: bool,
  startDate: { type: "string", format: "date" },
  endDate: { type: "string", format: "date" },
  timezone: str,
});
const op = (summary, schema, parameters = []) => ({
  tags: ["Personalization"],
  summary,
  security: [{ sessionCookie: [] }],
  parameters,
  responses: {
    200: {
      description: "Success",
      content: { "application/json": { schema } },
    },
    400: { description: "Invalid input" },
    401: { description: "Sign in required" },
    403: { description: "Role or Origin/CSRF denied" },
  },
});
const mutation = (summary, input, output, params = []) => ({
  ...op(summary, output, [
    ...params,
    { $ref: "#/components/parameters/Origin" },
    { $ref: "#/components/parameters/CSRF" },
  ]),
  ...(input
    ? {
        requestBody: {
          required: true,
          content: { "application/json": { schema: input } },
        },
      }
    : {}),
});
const ok = obj({ ok: bool });
d.paths["/users/me/channels"] = {
  get: op(
    "Own enabled channels, including personally hidden channels",
    ref("PersonalChannels"),
  ),
};
d.paths["/users/me/channels/order"] = {
  patch: mutation(
    "Save personal browse mode and optional full channel permutation",
    obj({ mode: s.PersonalChannels.properties.mode, channelIds: arr(str) }),
    ok,
  ),
};
d.paths["/users/me/channels/{id}/hide"] = {
  post: mutation(
    "Hide from own browsing; exact numeric tuning still works",
    null,
    ok,
    [id],
  ),
  delete: mutation("Show in own browsing", null, ok, [id]),
};
d.paths["/users/me/channels/reset"] = {
  post: mutation("Reset only order and visibility; retain favorites", null, ok),
};
d.paths["/users/me/history"] = {
  get: op(
    "Own viewing history, retained for 180 days; pages of 50",
    obj({ items: arr(ref("ViewingHistory")), retentionDays: num }),
    [
      {
        name: "offset",
        in: "query",
        schema: { type: "integer", minimum: 0, maximum: 100000 },
      },
    ],
  ),
  delete: mutation(
    "Clear own history and derived affinities, preserving favorites/preferences and schedules",
    null,
    ok,
  ),
};
for (const name of ["for-you", "whats-good"])
  d.paths["/personalization/" + name] = {
    get: op(
      "Rank existing current/next broadcasts locally; personal signals only",
      ref("Recommendations"),
      [{ name: "debug", in: "query", schema: bool }],
    ),
  };
d.paths["/personalization/affinities"] = {
  get: op(
    "Own rebuildable affinities; never another user's history",
    obj({ items: arr(ref("ContentAffinity")), rankingVersion: num }),
  ),
};
d.paths["/personalization/rebuild"] = {
  post: mutation("Queue own affinity rebuild", null, obj({ jobId: str })),
};
d.paths["/personalization/rebuild"].post.responses[202] =
  d.paths["/personalization/rebuild"].post.responses[200];
delete d.paths["/personalization/rebuild"].post.responses[200];
d.paths["/smart-channel-templates"] = {
  get: op(
    "Owner/admin shared-channel template gallery",
    obj({ items: arr(ref("SmartTemplate")) }),
  ),
};
d.paths["/smart-channel-templates/preview"] = {
  post: mutation(
    "Owner/admin preview through existing local resolver",
    ref("SmartTemplateInput"),
    obj({
      preview: ref("ChannelPreview"),
      source: ref("ChannelSource"),
      scheduleMode: str,
      presentation: str,
    }),
  ),
};
d.paths["/smart-channel-templates/instantiate"] = {
  post: mutation(
    "Owner/admin create normal shared channel and queue its schedule",
    ref("SmartTemplateInput"),
    ref("Channel"),
  ),
};
d.paths["/smart-channel-templates/instantiate"].post.responses[201] =
  d.paths["/smart-channel-templates/instantiate"].post.responses[200];
delete d.paths["/smart-channel-templates/instantiate"].post.responses[200];
d.paths["/channels/{id}/activation"] = {
  get: op(
    "Owner/admin inspect explicit seasonal dates",
    ref("ActivationWindow"),
    [id],
  ),
  patch: mutation(
    "Owner/admin update inclusive local-date window and queue future repair",
    obj({
      startDate: { type: "string", format: "date" },
      endDate: { type: "string", format: "date" },
      timezone: str,
    }),
    ok,
    [id],
  ),
  delete: mutation(
    "Owner/admin restore always-active scheduling and queue repair",
    null,
    ok,
    [id],
  ),
};
d.paths["/watch/channel-number/{number}"] = {
  get: op(
    "Resolve an enabled global number, independent of personal order/hiding",
    obj({ channelId: str }),
    [{ name: "number", in: "path", required: true, schema: str }],
  ),
};
fs.writeFileSync(file, YAML.stringify(d, { lineWidth: 100 }));
