// Adds the Phase 2 local membership contract without changing Plex routes.
import fs from "node:fs";
import { createRequire } from "node:module";
const require = createRequire(new URL("../web/package.json", import.meta.url));
const YAML = require("yaml");
const file = new URL("../openapi.yaml", import.meta.url);
const doc = YAML.parse(fs.readFileSync(file, "utf8"));
doc.info.version = "0.3.0";
doc.info.description =
  "Self-hosted Plex inventory and virtual channel membership. Channel, Plex, media and jobs routes require OWNER or ADMIN. Mutations require configured Origin and session X-CSRF-Token. No scheduling, EPG or playback APIs.";
if (!doc.tags.some((t) => t.name === "Channels"))
  doc.tags.push({ name: "Channels" });
const ref = (n) => ({ $ref: `#/components/schemas/${n}` });
const str = { type: "string" },
  num = { type: "integer" },
  bool = { type: "boolean" },
  nullable = { type: "string", nullable: true };
const arr = (items) => ({ type: "array", items });
const obj = (properties, required = []) => ({
  type: "object",
  additionalProperties: false,
  properties,
  required,
});
const schemas = doc.components.schemas;
schemas.ChannelIssue = obj({ path: str, message: str });
schemas.ChannelError = obj({
  code: str,
  error: str,
  issues: arr(ref("ChannelIssue")),
});
schemas.RuleNode = {
  description:
    "Recursive version 1 condition/group. Maximum 8 levels, 64 nodes. Groups have AND/OR/NOT and children; NOT exactly one child. Conditions have field/operator/value; boolean operators omit value. SQL identifiers are never accepted.",
  oneOf: [
    obj(
      {
        type: { type: "string", enum: ["group"] },
        operator: { type: "string", enum: ["AND", "OR", "NOT"] },
        children: { ...arr(ref("RuleNode")), minItems: 1, maxItems: 64 },
      },
      ["type", "operator", "children"],
    ),
    obj(
      {
        type: { type: "string", enum: ["condition"] },
        field: str,
        operator: str,
        value: {
          oneOf: [str, { type: "number" }, arr(str), arr({ type: "number" })],
        },
      },
      ["type", "field", "operator"],
    ),
  ],
};
schemas.ChannelRule = obj(
  { version: { type: "integer", enum: [1] }, root: ref("RuleNode") },
  ["version", "root"],
);
schemas.ChannelSource = {
  description:
    "Exactly one configuration matching type. Static IDs reference playable inventory and preserve order. Collection ID is entityId@libraryId, from metadata discovery.",
  oneOf: [
    obj(
      {
        type: { type: "string", enum: ["STATIC"] },
        static: obj(
          { mediaIds: { ...arr(str), maxItems: 1000, uniqueItems: true } },
          ["mediaIds"],
        ),
      },
      ["type", "static"],
    ),
    obj(
      {
        type: { type: "string", enum: ["SHOW"] },
        show: obj(
          {
            showId: str,
            episodeScope: { type: "string", enum: ["ALL", "SELECTED_SEASONS"] },
            seasonIds: { ...arr(str), maxItems: 200 },
            includeSpecials: bool,
            excludeUnavailable: { ...bool, default: true },
          },
          ["showId", "episodeScope"],
        ),
      },
      ["type", "show"],
    ),
    obj(
      {
        type: { type: "string", enum: ["COLLECTION"] },
        collection: obj({ collectionId: str }, ["collectionId"]),
      },
      ["type", "collection"],
    ),
    obj(
      { type: { type: "string", enum: ["RULE"] }, rule: ref("ChannelRule") },
      ["type", "rule"],
    ),
  ],
};
schemas.ChannelAppearance = obj(
  {
    source: { type: "string", enum: ["AUTO", "PLEX", "UPLOAD"] },
    uploadId: nullable,
  },
  ["source"],
);
schemas.ChannelDefinition = obj(
  {
    number: { type: "string", pattern: "^[A-Za-z0-9][A-Za-z0-9._-]{0,23}$" },
    name: { ...str, maxLength: 100 },
    shortName: { ...str, maxLength: 24 },
    description: { ...str, maxLength: 2000 },
    groupId: nullable,
    enabled: bool,
    visibility: { type: "string", enum: ["VISIBLE", "HIDDEN"] },
    sortOrder: num,
    logo: ref("ChannelAppearance"),
    background: ref("ChannelAppearance"),
    artworkMediaId: nullable,
    source: ref("ChannelSource"),
  },
  ["number", "name", "visibility", "logo", "background", "source"],
);
schemas.Channel = obj({
  ...schemas.ChannelDefinition.properties,
  id: str,
  count: num,
  warnings: arr(str),
  createdAt: num,
  updatedAt: num,
});
schemas.ChannelGroup = obj({
  id: str,
  name: str,
  slug: str,
  description: str,
  sortOrder: num,
  system: bool,
  count: num,
});
schemas.ChannelMedia = obj({
  id: str,
  title: str,
  type: { type: "string", enum: ["movie", "episode"] },
  year: { ...num, nullable: true },
  showTitle: str,
  season: { ...num, nullable: true },
  episode: { ...num, nullable: true },
  durationMs: { ...num, nullable: true },
  library: str,
  available: bool,
  eligible: bool,
});
schemas.ChannelPreview = obj({
  count: num,
  items: arr(ref("ChannelMedia")),
  offset: num,
  limit: num,
  warnings: arr(str),
});
schemas.ChannelPreviewRequest = obj(
  {
    source: ref("ChannelSource"),
    includeUnavailable: { ...bool, default: false },
    offset: { ...num, minimum: 0 },
    limit: { ...num, minimum: 1, maximum: 100, default: 24 },
  },
  ["source"],
);
schemas.ChannelMetadata = obj({
  id: str,
  name: str,
  context: str,
  count: num,
  artworkId: str,
  seasons: num,
  episodes: num,
});
schemas.ChannelField = obj({
  name: str,
  label: str,
  kind: {
    type: "string",
    enum: ["reference", "enum", "string", "number", "date", "boolean"],
  },
  operators: arr(str),
});
const json = (s) => ({ "application/json": { schema: s } });
const q = (name, schema = str) => ({ name, in: "query", schema });
function endpoint(
  method,
  path,
  summary,
  result,
  body,
  status = 200,
  parameters = [],
) {
  const params = [...path.matchAll(/\{([^}]+)\}/g)].map((m) => ({
    name: m[1],
    in: "path",
    required: true,
    schema: str,
  }));
  if (method !== "get")
    params.push(
      { name: "X-CSRF-Token", in: "header", required: true, schema: str },
      { name: "Origin", in: "header", required: true, schema: str },
    );
  const op = {
    tags: ["Channels"],
    summary,
    security: [{ sessionCookie: [] }],
    parameters: [...params, ...parameters],
    responses: {
      [status]: { description: "Success", content: json(result) },
      401: { description: "Sign in required" },
      403: { description: "Administrator role, Origin or CSRF rejected" },
      404: { description: "Not found" },
      409: { description: "Duplicate number or slug" },
      422: {
        description: "Invalid source or structured rule issues",
        content: json(ref("ChannelError")),
      },
    },
  };
  if (body) op.requestBody = { required: true, content: json(body) };
  (doc.paths[path] ??= {})[method] = op;
  return op;
}
const paging = [q("offset", num), q("limit", { ...num, maximum: 100 })];
endpoint(
  "get",
  "/channels",
  "List channels and counts from local inventory",
  obj({ items: arr(ref("Channel")), more: bool, nextNumber: str }),
  undefined,
  200,
  [
    ...paging,
    q("group", {
      ...str,
      description: "Group ID or ungrouped; omitted means all",
    }),
  ],
);
endpoint(
  "post",
  "/channels",
  "Create channel",
  ref("Channel"),
  ref("ChannelDefinition"),
  201,
);
endpoint("get", "/channels/{id}", "Read channel", ref("Channel"));
endpoint(
  "patch",
  "/channels/{id}",
  "Replace complete editable channel definition",
  ref("Channel"),
  ref("ChannelDefinition"),
);
endpoint(
  "delete",
  "/channels/{id}",
  "Delete channel, retaining Plex inventory",
  obj({ deleted: bool }),
);
endpoint(
  "patch",
  "/channels/{id}/enabled",
  "Set enabled state",
  obj({ saved: bool }),
  obj({ enabled: bool }, ["enabled"]),
);
endpoint(
  "post",
  "/channels/{id}/duplicate",
  "Duplicate source, artwork and group with a new identity and number",
  ref("Channel"),
  undefined,
  201,
);
endpoint(
  "get",
  "/channels/{id}/media",
  "Resolve deterministic membership, not a schedule",
  ref("ChannelPreview"),
  undefined,
  200,
  [...paging, q("includeUnavailable", bool)],
);
endpoint(
  "get",
  "/channels/{id}/media/count",
  "Count playable matching inventory",
  obj({ count: num }),
);
endpoint(
  "post",
  "/channel-preview",
  "Preview any source without saving",
  ref("ChannelPreview"),
  ref("ChannelPreviewRequest"),
);
endpoint(
  "post",
  "/channel-rules/validate",
  "Validate a versioned rule",
  obj({ valid: bool }),
  ref("ChannelRule"),
);
endpoint(
  "post",
  "/channel-rules/preview",
  "Preview rule matches",
  ref("ChannelPreview"),
  obj(
    {
      rule: ref("ChannelRule"),
      offset: num,
      limit: num,
      includeUnavailable: bool,
    },
    ["rule"],
  ),
);
endpoint(
  "get",
  "/channel-rules/fields",
  "Discover supported rule fields and operators",
  arr(ref("ChannelField")),
);
endpoint(
  "get",
  "/channel-metadata/{field}",
  "Search bounded metadata options with source context",
  obj({ items: arr(ref("ChannelMetadata")), more: bool }),
  undefined,
  200,
  [
    q("q"),
    q("parent", { ...str, description: "Local show ID for season selection" }),
    ...paging,
  ],
);
endpoint(
  "get",
  "/channel-metadata/selected",
  "Resolve one saved metadata selection",
  ref("ChannelMetadata"),
  undefined,
  200,
  [q("field"), q("value")],
);
endpoint(
  "get",
  "/channel-groups",
  "List groups in display order",
  arr(ref("ChannelGroup")),
);
endpoint(
  "post",
  "/channel-groups",
  "Create group",
  ref("ChannelGroup"),
  ref("ChannelGroup"),
  201,
);
endpoint(
  "patch",
  "/channel-groups/{id}",
  "Edit group and sort order",
  ref("ChannelGroup"),
  ref("ChannelGroup"),
);
endpoint(
  "delete",
  "/channel-groups/{id}",
  "Delete group; explicitly move or ungroup contained channels",
  obj({ deleted: bool }),
  obj({ moveTo: nullable, ungroup: bool }),
);
const upload = endpoint(
  "post",
  "/channel-artwork",
  "Upload PNG/JPEG/WebP, at most 8 MiB, 8192 pixels per edge and 36 megapixels",
  obj({ id: str, contentType: str, width: num, height: num }),
);
upload.requestBody = {
  required: true,
  content: Object.fromEntries(
    ["image/png", "image/jpeg", "image/webp"].map((m) => [
      m,
      { schema: { type: "string", format: "binary" } },
    ]),
  ),
};
upload.responses["201"] = upload.responses["200"];
delete upload.responses["200"];
for (const path of ["/channel-artwork/{id}", "/channels/{id}/artwork/{kind}"]) {
  const op = endpoint(
    "get",
    path,
    "Authenticated channel artwork; kind is logo or background",
    str,
  );
  op.responses["200"].content = Object.fromEntries(
    ["image/png", "image/jpeg", "image/webp", "image/svg+xml"].map((m) => [
      m,
      { schema: { type: "string", format: "binary" } },
    ]),
  );
}
fs.writeFileSync(file, YAML.stringify(doc, { lineWidth: 100 }));
