import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { guideAPI, type WindowData, type Detail } from "./api";
import { GuideGrid } from "./GuideGrid";
import { ProgramDetails } from "./ProgramDetails";
import { MiniGuide } from "./MiniGuide";
import { geometry, dayStart, stateAt } from "./timeline";
vi.mock("./api", () => ({
  guideAPI: {
    window: vi.fn(),
    now: vi.fn(),
    detail: vi.fn(),
    favorite: vi.fn(),
    favorites: vi.fn(),
    soon: vi.fn(),
    last: vi.fn(),
  },
}));
const now = Date.UTC(2026, 9, 17, 22, 0);
const program = (id: string, start: number, end: number) => ({
  id,
  channelId: "a",
  mediaId: "m",
  title: id,
  showTitle: "",
  type: "movie",
  start,
  end,
  durationMs: end - start,
  season: null,
  episode: null,
  available: true,
  artwork: "/poster",
  state: stateAt({ start, end }, now) as "PAST" | "CURRENT" | "FUTURE",
  progress: 50,
  canRestart: start <= now,
});
const current = program("Current film", now - 600000, now + 1200000),
  past = program("Past film", now - 3600000, now - 600000),
  future = program("Future film", now + 1200000, now + 3600000);
const data: WindowData = {
  channels: [
    {
      id: "a",
      number: "101",
      name: "Cinema",
      logo: "/logo",
      groupId: "movies",
      group: "Movies",
      favorite: false,
      status: "READY",
      recentKind: "",
      programs: [past, current, future],
    },
    {
      id: "b",
      number: "102",
      name: "TV",
      logo: "/logo",
      groupId: "tv",
      group: "TV",
      favorite: false,
      status: "READY",
      recentKind: "TV",
      programs: [
        { ...current, id: "Episode", title: "Episode", channelId: "b" },
      ],
    },
  ],
  groups: [{ id: "movies", name: "Movies" }],
  serverTime: now,
  timezone: "America/New_York",
  more: false,
  start: now,
  end: now + 10800000,
  warnings: [],
};
const detail = (p = current): Detail => ({
  ...p,
  channel: data.channels[0],
  summary: "A real description",
  year: 2026,
  rating: 8,
  contentRating: "PG",
  addedAt: null,
  genres: ["Drama"],
  background: "/backdrop",
  serverTime: now,
  timezone: "America/New_York",
});
beforeEach(() => {
  vi.mocked(guideAPI.window).mockResolvedValue(structuredClone(data));
  vi.mocked(guideAPI.now).mockResolvedValue(structuredClone(data));
  vi.mocked(guideAPI.detail).mockResolvedValue(detail());
  vi.mocked(guideAPI.favorite).mockResolvedValue({});
  HTMLElement.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
it("maps exact durations and clips intersecting programs", () => {
  expect(geometry(0, 22 * 60000, 0, 3 * 3600000).width).toBe(132);
  expect(geometry(-60000, 60000, 0, 3600000)).toEqual({ left: 0, width: 6 });
  expect(stateAt({ start: 10, end: 20 }, 20)).toBe("PAST");
  expect(stateAt({ start: 10, end: 20 }, 10)).toBe("CURRENT");
});
it("resolves DST calendar midnights without assuming 24-hour days", () => {
  for (const [date, hours] of [
    ["2026-03-08T12:00:00Z", 23],
    ["2026-11-01T12:00:00Z", 25],
  ] as const) {
    const t = Date.parse(date);
    expect(
      dayStart(t, "America/New_York", 1) - dayStart(t, "America/New_York"),
    ).toBe(hours * 3600000);
  }
});
it("renders channels, true-width cells, active state and now line", async () => {
  render(<GuideGrid csrf="x" userId="render" onTune={vi.fn()} />);
  expect(await screen.findByRole("grid")).toBeInTheDocument();
  expect(screen.getByText("Cinema")).toBeInTheDocument();
  expect(screen.getByRole("gridcell", { name: /Current film/ })).toHaveClass(
    "current",
  );
  expect(screen.getByRole("gridcell", { name: /Past film/ })).toHaveClass(
    "past",
  );
  expect(screen.getByRole("gridcell", { name: /Future film/ })).toHaveClass(
    "future",
  );
  expect(screen.getByLabelText("Current time")).toBeInTheDocument();
});
it("navigates horizontally/vertically and restores focus after details", async () => {
  render(<GuideGrid csrf="x" userId="navigation" onTune={vi.fn()} />);
  const currentCell = await screen.findByRole("gridcell", {
    name: /Current film/,
  });
  currentCell.focus();
  fireEvent.keyDown(currentCell, { key: "ArrowDown" });
  await waitFor(() =>
    expect(screen.getByRole("gridcell", { name: /Episode/ })).toHaveFocus(),
  );
  fireEvent.keyDown(document.activeElement!, { key: "ArrowUp" });
  await waitFor(() => expect(currentCell).toHaveFocus());
  fireEvent.keyDown(currentCell, { key: "ArrowRight" });
  await waitFor(() =>
    expect(screen.getByRole("gridcell", { name: /Future film/ })).toHaveFocus(),
  );
  const futureCell = document.activeElement!;
  fireEvent.keyDown(futureCell, { key: "Enter" });
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  await waitFor(() => expect(futureCell).toHaveFocus());
});
it("changes day, jumps now and filters groups/favorites", async () => {
  render(<GuideGrid csrf="x" userId="dates" onTune={vi.fn()} />);
  await screen.findByRole("grid");
  fireEvent.click(screen.getByText("Yesterday"));
  await waitFor(() =>
    expect(vi.mocked(guideAPI.window).mock.calls.at(-1)![0]).toBeLessThan(now),
  );
  fireEvent.click(screen.getByRole("button", { name: "NOW" }));
  await waitFor(() =>
    expect(vi.mocked(guideAPI.window).mock.calls.at(-1)![0]).toBe(now),
  );
  fireEvent.click(screen.getByText("Movies"));
  await waitFor(() =>
    expect(vi.mocked(guideAPI.window).mock.calls.at(-1)![2]).toBe("movies"),
  );
  fireEvent.click(screen.getByText("★ Favorites"));
  await waitFor(() =>
    expect(vi.mocked(guideAPI.window).mock.calls.at(-1)![3]).toBe(true),
  );
});
it.each([
  ["current", current, true, true],
  ["past", past, false, true],
  ["future", future, false, false],
] as const)(
  "details %s actions enforce broadcast state",
  async (_name, p, live, restart) => {
    vi.mocked(guideAPI.detail).mockResolvedValue(detail(p));
    const tune = vi.fn();
    render(
      <ProgramDetails id={p.id} csrf="x" onClose={vi.fn()} onTune={tune} />,
    );
    await screen.findByText("A real description");
    expect(!!screen.queryByText("Watch Live")).toBe(live);
    expect(!!screen.queryByText("Watch From Beginning")).toBe(restart);
    if (restart) {
      fireEvent.click(screen.getByText("Watch From Beginning"));
      expect(tune).toHaveBeenCalledWith("a", p.id);
    }
    if (live) {
      fireEvent.click(screen.getByText("Watch Live"));
      expect(tune).toHaveBeenCalledWith("a");
    }
  },
);
it("favorites are persisted through personal API", async () => {
  render(
    <ProgramDetails id="a" csrf="csrf" onClose={vi.fn()} onTune={vi.fn()} />,
  );
  fireEvent.click(await screen.findByText("☆ Favorite Channel"));
  await waitFor(() =>
    expect(guideAPI.favorite).toHaveBeenCalledWith("a", true, "csrf"),
  );
  expect(await screen.findByText("★ Unfavorite Channel")).toBeInTheDocument();
});
it("mini-guide browses/tunes and closes without owning a stream", async () => {
  const tune = vi.fn(),
    close = vi.fn();
  render(<MiniGuide channelId="a" onClose={close} onTune={tune} />);
  const b = await screen.findByRole("button", { name: /101 · Cinema/ });
  await waitFor(() => expect(b).toHaveFocus());
  fireEvent.keyDown(b, { key: "ArrowDown" });
  expect(screen.getByRole("button", { name: /102 · TV/ })).toHaveFocus();
  fireEvent.click(document.activeElement!);
  expect(tune).toHaveBeenCalledWith("b");
  fireEvent.keyDown(screen.getByRole("dialog"), { key: "Escape" });
  expect(close).toHaveBeenCalled();
});
it("shows honest no-favorites empty state", async () => {
  vi.mocked(guideAPI.window).mockResolvedValue({ ...data, channels: [] });
  render(<GuideGrid csrf="x" userId="empty" onTune={vi.fn()} />);
  fireEvent.click(await screen.findByText("★ Favorites"));
  expect(
    await screen.findByText("Your favorites start here"),
  ).toBeInTheDocument();
});

it("home shows server last channel, favorites and time-sorted starting soon", async () => {
  const { ViewerHome } = await import("../playback/ViewerHome");
  const { playbackAPI } = await import("../playback/api");
  vi.spyOn(playbackAPI, "onNow").mockResolvedValue({
    serverTime: now,
    items: data.channels.map((c) => ({
      channel: { ...c, timezone: data.timezone },
      program: { ...current, policyReason: "test" },
      next: null,
      progress: 50,
    })),
  });
  vi.mocked(guideAPI.last).mockResolvedValue({ channelId: "b" });
  vi.mocked(guideAPI.favorites).mockResolvedValue({ channelIds: ["a"] });
  vi.mocked(guideAPI.soon).mockResolvedValue({
    ...data,
    channels: [{ ...data.channels[0], programs: [future] }],
  });
  render(<ViewerHome userId="home" csrf="x" fallback={<p>Setup</p>} />);
  expect(
    await screen.findByRole("button", { name: "Watch Last Channel" }),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("heading", { name: "Favorite Channels" }),
  ).toBeInTheDocument();
  expect(
    await screen.findByRole("heading", { name: "Starting Soon" }),
  ).toBeInTheDocument();
  expect(await screen.findByText("Future film")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Watch Last Channel" }));
  expect(window.location.hash).toBe("#watch/b");
});

it("unfavoriting the final filtered row leaves focus on NOW", async () => {
  vi.mocked(guideAPI.window).mockResolvedValue({
    ...data,
    channels: [{ ...data.channels[0], favorite: true }],
  });
  render(<GuideGrid csrf="x" userId="favorite-focus" onTune={vi.fn()} />);
  await screen.findByRole("grid");
  fireEvent.click(screen.getByText("★ Favorites"));
  await waitFor(() =>
    expect(vi.mocked(guideAPI.window).mock.calls.at(-1)![3]).toBe(true),
  );
  vi.mocked(guideAPI.window).mockResolvedValue({ ...data, channels: [] });
  fireEvent.click(screen.getByRole("button", { name: "★ Unfavorite Channel" }));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "NOW" })).toHaveFocus(),
  );
  expect(screen.getByText("Your favorites start here")).toBeInTheDocument();
});
