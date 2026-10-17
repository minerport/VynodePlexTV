// Extend OpenAPI with persisted scheduling contracts; no playback routes.
import fs from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml");
const file = new URL("../openapi.yaml", import.meta.url);
const doc = YAML.parse(fs.readFileSync(file, "utf8"));
doc.info.version = "0.4.0";
doc.info.description =
  "Self-hosted Plex inventory, virtual channel membership and persisted linear scheduling. Administrative mutations require OWNER/ADMIN, configured Origin and X-CSRF-Token. Authenticated users may read enabled-channel schedules. No playback or full EPG APIs.";
doc.tags = doc.tags.filter((t) => t.name !== "Scheduling");
doc.tags.push({ name: "Scheduling" });
const ref = (n) => ({ $ref: `#/components/schemas/${n}` });
const str = { type: "string" },
  int = { type: "integer" },
  ms = {
    type: "integer",
    format: "int64",
    description: "UTC Unix milliseconds",
  },
  bool = { type: "boolean" };
const obj = (properties) => ({ type: "object", properties });
const arr = (items) => ({ type: "array", items });
const schemas = doc.components.schemas;
schemas.ScheduleSettings = obj({
  channelId: str,
  mode: { type: "string", enum: ["SEQUENTIAL", "RANDOM", "SMART_RANDOM"] },
  timezone: { type: "string", description: "IANA timezone" },
  anchorMs: ms,
  seed: { type: "string", minLength: 1, maxLength: 128 },
  movieRepeatDays: { ...int, minimum: 0, maximum: 365 },
  episodeRepeatDays: { ...int, minimum: 0, maximum: 365 },
  pastDays: { ...int, minimum: 0, maximum: 30 },
  futureDays: { ...int, minimum: 1, maximum: 30 },
  includeUnavailable: bool,
  enabled: bool,
});
schemas.ScheduleSettings.required = Object.keys(
  schemas.ScheduleSettings.properties,
).filter((x) => x !== "channelId");
schemas.ScheduleSettings.additionalProperties = false;
schemas.ScheduledProgram = obj({
  id: str,
  channelId: str,
  mediaId: str,
  start: ms,
  end: ms,
  durationMs: int,
  sequence: int,
  revision: int,
  policyReason: str,
  title: str,
  showTitle: str,
  season: { ...int, nullable: true },
  episode: { ...int, nullable: true },
  available: bool,
  artwork: {
    ...str,
    description: "Authenticated admin artwork proxy; never a Plex URL/token",
  },
});
schemas.ScheduledProgram.description =
  "Persisted interval [start,end). Current and past rows retain original timing; display metadata is read from current local inventory.";
schemas.SchedulePage = obj({
  items: arr(ref("ScheduledProgram")),
  more: bool,
  start: ms,
  end: ms,
  timezone: str,
});
schemas.ScheduleStatus = obj({
  channelId: str,
  name: str,
  number: str,
  mode: str,
  timezone: str,
  status: {
    type: "string",
    enum: [
      "READY",
      "EMPTY",
      "NEEDS_GENERATION",
      "NEEDS_REPAIR",
      "GENERATING",
      "ERROR",
      "DISABLED",
    ],
  },
  resolved: int,
  schedulable: int,
  excluded: int,
  coverageStart: { ...ms, nullable: true },
  coverageEnd: { ...ms, nullable: true },
  programCount: int,
  lastGenerated: { ...ms, nullable: true },
  lastError: str,
  integrity: arr(str),
  warnings: arr(str),
  revision: int,
});
schemas.ScheduleJob = obj({
  id: str,
  channelId: str,
  type: str,
  status: str,
  processed: int,
  repaired: int,
  error: str,
  createdAt: { ...int, description: "Unix seconds" },
  startedAt: { ...int, nullable: true },
  completedAt: { ...int, nullable: true },
  affectedStart: { ...ms, nullable: true },
  affectedEnd: { ...ms, nullable: true },
});
const param = (name, schema, required = false, where = "query") => ({
  name,
  in: where,
  required,
  schema,
});
const id = param("id", str, true, "path");
const response = (schema) => ({
  description: "Success",
  content: { "application/json": { schema } },
});
const op = (summary, schema, parameters = []) => ({
  tags: ["Scheduling"],
  summary,
  security: [{ sessionCookie: [] }],
  parameters,
  responses: {
    200: response(schema),
    401: { description: "Authentication required" },
    403: { description: "Administrator role or CSRF/origin required" },
    404: { description: "Channel not found or not visible" },
    422: { description: "Invalid settings, timestamp or interval" },
  },
});
doc.paths["/scheduler"] = {
  get: op("Admin paginated schedule diagnostics", arr(ref("ScheduleStatus")), [
    param("offset", int),
    param("limit", { ...int, maximum: 100 }),
  ]),
};
doc.paths["/scheduler/jobs"] = {
  get: op(
    "Admin latest 30 schedule jobs; progress is indeterminate while computing",
    arr(ref("ScheduleJob")),
    [param("channel", str)],
  ),
};
const base = "/channels/{id}/schedule";
doc.paths[base] = {
  get: op(
    "Read enabled-channel timeline using day OR explicit start/end",
    ref("SchedulePage"),
    [
      id,
      param("day", {
        type: "string",
        format: "date",
        description: "Calendar day in channel timezone, including DST",
      }),
      param("start", { type: "string", format: "date-time" }),
      param("end", { type: "string", format: "date-time" }),
      param("offset", int),
      param("limit", { ...int, maximum: 500 }),
    ],
  ),
};
doc.paths[base + "/at"] = {
  get: op(
    "Current, next and previous at a timestamp (default now)",
    obj({
      current: { ...ref("ScheduledProgram"), nullable: true },
      next: { ...ref("ScheduledProgram"), nullable: true },
      previous: { ...ref("ScheduledProgram"), nullable: true },
    }),
    [id, param("time", { type: "string", format: "date-time" })],
  ),
};
doc.paths[base + "/status"] = {
  get: op("Admin integrity and coverage diagnostics", ref("ScheduleStatus"), [
    id,
  ]),
};
const patch = op(
  "Admin replace scheduling settings; future maintenance applies changes",
  ref("ScheduleSettings"),
  [id],
);
patch.requestBody = {
  required: true,
  content: { "application/json": { schema: ref("ScheduleSettings") } },
};
doc.paths[base + "/settings"] = {
  get: op("Admin persisted settings", ref("ScheduleSettings"), [id]),
  patch,
};
for (const action of ["generate", "repair", "regenerate"]) {
  const post = op(
    `Admin queue ${action}; preserve all programs that have started`,
    obj({ jobId: str }),
    [id],
  );
  post.responses[202] = post.responses[200];
  delete post.responses[200];
  doc.paths[base + "/" + action] = { post };
}
// Use the existing cookie security scheme name.
const scheme = Object.keys(doc.components.securitySchemes).find(
  (k) => doc.components.securitySchemes[k].in === "cookie",
);
for (const [p, methods] of Object.entries(doc.paths)) {
  if (p.startsWith("/scheduler") || p.includes("/schedule"))
    for (const o of Object.values(methods))
      if (o.security) o.security = [{ [scheme]: [] }];
}
fs.writeFileSync(file, YAML.stringify(doc, { lineWidth: 100 }));
