// Test-only Plex HTTP fixture. Never used by the application image or normal Compose stack.
import http from "node:http";
import process from "node:process";
import { URL } from "node:url";
import { Buffer } from "node:buffer";
const token = process.env.PLEX_FIXTURE_TOKEN;
if (!token) throw new Error("A generated test token is required");
const xml = (response, body) => {
  response.writeHead(200, { "content-type": "application/xml" });
  response.end(body);
};
http
  .createServer((request, response) => {
    const url = new URL(request.url, "http://fixture");
    if (url.pathname === "/health") {
      response.end("ok");
      return;
    }
    if (request.headers["x-plex-token"] !== token) {
      response.writeHead(401);
      response.end("Unauthorized");
      return;
    }
    if (url.pathname === "/") {
      xml(
        response,
        '<MediaContainer machineIdentifier="e2e-fixture" friendlyName="Test Cinema" product="Plex Media Server" version="1.0" platform="Linux"/>',
      );
      return;
    }
    if (url.pathname === "/library/sections") {
      xml(
        response,
        '<MediaContainer size="2"><Directory key="1" type="movie" title="Test Movies" uuid="movie" updatedAt="1"/><Directory key="2" type="show" title="Test TV" uuid="tv" updatedAt="1"/></MediaContainer>',
      );
      return;
    }
    const kind = url.searchParams.get("type");
    const types = {
      1: ["10", "movie", "Test Movie"],
      2: ["20", "show", "Test Show"],
      3: ["21", "season", "Season One"],
      4: ["22", "episode", "Test Episode"],
    };
    const metadata = (key, type, title) =>
      `<${type === "movie" || type === "episode" ? "Video" : "Directory"} ratingKey="${key}" type="${type}" title="${title}" year="2024" updatedAt="1" thumb="/library/metadata/${key}/thumb/1" art="/library/metadata/${key}/art/1" ${type === "season" ? 'parentRatingKey="20" index="1"' : type === "episode" ? 'parentRatingKey="21" grandparentRatingKey="20" index="1" parentIndex="1"' : ""}><Genre id="1" tag="Drama"/><Collection id="2" tag="Test Universe"/><Guid id="tmdb://1"/></${type === "movie" || type === "episode" ? "Video" : "Directory"}>`;
    if (url.pathname.endsWith("/all")) {
      const record = types[kind];
      xml(
        response,
        `<MediaContainer size="1" totalSize="1" offset="0">${metadata(...record)}</MediaContainer>`,
      );
      return;
    }
    if (url.pathname.includes("/thumb/") || url.pathname.includes("/art/")) {
      response.writeHead(200, { "content-type": "image/png" });
      response.end(
        Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aV1sAAAAASUVORK5CYII=",
          "base64",
        ),
      );
      return;
    }
    const record = Object.values(types).find(
      (item) => url.pathname === `/library/metadata/${item[0]}`,
    );
    if (record) {
      xml(
        response,
        `<MediaContainer size="1">${metadata(...record)}</MediaContainer>`,
      );
      return;
    }
    response.writeHead(404);
    response.end();
  })
  .listen(32400, "0.0.0.0");
