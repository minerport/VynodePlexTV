import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PlayerController } from "./controller";
import { playbackAPI, type Tune } from "./api";
import { ViewerHome } from "./ViewerHome";
const fixture: Tune = {
  id: "session",
  channel: {
    id: "channel",
    number: "107",
    name: "Recently Added Movies",
    logo: "/art",
    recentKind: "MOVIES",
  },
  program: {
    id: "p",
    mediaId: "m",
    title: "Movie",
    showTitle: "",
    season: null,
    episode: null,
    start: 100000,
    end: 3700000,
    durationMs: 3600000,
    available: true,
    artwork: "/art",
    policyReason: "test",
  },
  next: {
    id: "next",
    mediaId: "n",
    title: "Next movie",
    showTitle: "",
    season: null,
    episode: null,
    start: 3700000,
    end: 7300000,
    durationMs: 3600000,
    available: true,
    artwork: "/art",
    policyReason: "test",
  },
  mode: "LIVE",
  state: "STARTING",
  serverTime: 760000,
  liveOffsetMs: 660000,
  positionMs: 660000,
  durationMs: 3600000,
  streamUrl: "/api/v1/playback/session/stream/main",
  mime: "video/mp4",
  delivery: "DIRECT_PLAY",
  addedAt: 1,
};
const controllers: PlayerController[] = [];
beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
  vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue(
    "probably",
  );
  vi.spyOn(playbackAPI, "stop").mockResolvedValue({});
  vi.spyOn(playbackAPI, "heartbeat").mockResolvedValue({
    serverTime: 760000,
    mode: "LIVE",
    liveOffsetMs: 660000,
    transition: false,
    ended: false,
  });
});
afterEach(() => {
  for (const c of controllers) c.dispose();
  controllers.length = 0;
  cleanup();
  vi.restoreAllMocks();
  window.location.hash = "";
});
function player() {
  const video = document.createElement("video");
  Object.defineProperty(video, "duration", { value: 3600 });
  const change = vi.fn();
  const c = new PlayerController(video, "csrf", change);
  controllers.push(c);
  return { video, c, change };
}
it("tunes live from the server offset and cleans up on exit", async () => {
  vi.spyOn(playbackAPI, "tune").mockResolvedValue(fixture);
  const { c, video } = player();
  await c.tune("channel");
  expect(c.state.loading).toBe(true);
  video.dispatchEvent(new Event("loadedmetadata"));
  expect(video.currentTime).toBeCloseTo(660, 0);
  video.dispatchEvent(new Event("playing"));
  expect(c.state.loading).toBe(false);
  expect(c.state.tune?.next?.title).toBe("Next movie");
  c.dispose();
  expect(playbackAPI.stop).toHaveBeenCalledWith("session", "csrf");
});
it("restarts from beginning and Go Live uses a fresh server context", async () => {
  vi.spyOn(playbackAPI, "tune").mockResolvedValue({
    ...fixture,
    mode: "FROM_BEGINNING",
    positionMs: 0,
  });
  vi.spyOn(playbackAPI, "goLive").mockResolvedValue({
    ...fixture,
    id: "new-session",
  });
  const { c, video } = player();
  await c.tune("channel", "FROM_BEGINNING");
  video.dispatchEvent(new Event("loadedmetadata"));
  expect(video.currentTime).toBe(0);
  expect(c.state.tune?.mode).toBe("FROM_BEGINNING");
  await c.goLive();
  expect(playbackAPI.goLive).toHaveBeenCalledWith("session", "csrf");
  expect(c.state.tune?.id).toBe("new-session");
});
it("shows honest failure and retry resolves the current schedule again", async () => {
  const tune = vi
    .spyOn(playbackAPI, "tune")
    .mockRejectedValueOnce(new Error("Plex server unavailable"))
    .mockResolvedValue(fixture);
  const { c } = player();
  await c.tune("channel");
  expect(c.state.error).toBe("Plex server unavailable");
  await c.retry();
  expect(tune).toHaveBeenCalledTimes(2);
  expect(c.state.error).toBe("");
});
it("uses server behind-live state after pause or seek", async () => {
  vi.spyOn(playbackAPI, "tune").mockResolvedValue(fixture);
  vi.mocked(playbackAPI.heartbeat).mockResolvedValue({
    serverTime: 780000,
    mode: "BEHIND_LIVE",
    liveOffsetMs: 680000,
    transition: false,
    ended: false,
  });
  const { c, video } = player();
  await c.tune("channel");
  video.currentTime = 600;
  video.dispatchEvent(new Event("playing"));
  await waitFor(() => expect(c.state.tune?.mode).toBe("BEHIND_LIVE"));
});
it("transitions only when the server directs a live boundary", async () => {
  const tune = vi.spyOn(playbackAPI, "tune").mockResolvedValue(fixture);
  vi.mocked(playbackAPI.heartbeat).mockResolvedValueOnce({
    serverTime: 3700000,
    mode: "LIVE",
    liveOffsetMs: 3599999,
    transition: true,
    ended: false,
  });
  const { c, video } = player();
  await c.tune("channel");
  video.dispatchEvent(new Event("playing"));
  await waitFor(() => expect(tune).toHaveBeenCalledTimes(2));
});
it("discards a late tune response after channel switching", async () => {
  let resolve!: (v: Tune) => void;
  vi.spyOn(playbackAPI, "tune")
    .mockReturnValueOnce(
      new Promise((r) => {
        resolve = r;
      }),
    )
    .mockResolvedValue({ ...fixture, id: "second" });
  const { c } = player();
  const pending = c.tune("one");
  await c.tune("two");
  resolve(fixture);
  await pending;
  expect(c.state.tune?.id).toBe("second");
  expect(playbackAPI.stop).toHaveBeenCalledWith("session", "csrf");
});
it.each(["MOVIES", "TV"])(
  "offers Watch Live for Recently Added %s",
  async (kind) => {
    vi.spyOn(playbackAPI, "onNow").mockResolvedValue({
      serverTime: 760000,
      items: [
        {
          channel: {
            ...fixture.channel,
            recentKind: kind,
            name: `Recently Added ${kind}`,
          },
          program: fixture.program,
          next: fixture.next,
          progress: 18,
        },
      ],
    });
    render(<ViewerHome userId="u" fallback={<p>No channels</p>} />);
    fireEvent.click(await screen.findByText("Watch Live"));
    expect(window.location.hash).toBe("#watch/channel");
    expect(localStorage.getItem("afterglow-last-channel-u")).toBe("channel");
  },
);
it("preserves the no-channel setup experience", async () => {
  vi.spyOn(playbackAPI, "onNow").mockResolvedValue({
    items: [],
    serverTime: 0,
  });
  render(<ViewerHome userId="u" fallback={<p>Create your first channel</p>} />);
  expect(
    await screen.findByText("Create your first channel"),
  ).toBeInTheDocument();
});
