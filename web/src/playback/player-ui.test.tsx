import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { playbackAPI } from "./api";
const mock = vi.hoisted(() => ({
  tune: vi.fn(),
  goLive: vi.fn(),
  toggle: vi.fn(),
  retry: vi.fn(),
  dispose: vi.fn(),
}));
vi.mock("./controller", () => ({
  PlayerController: class {
    constructor(_v: unknown, _c: unknown, change: (v: unknown) => void) {
      queueMicrotask(() =>
        change({
          loading: false,
          error: "",
          needsPlay: false,
          message: "",
          tune: {
            id: "s",
            channel: { id: "a", number: "101", name: "Cinema", recentKind: "" },
            program: {
              title: "Film",
              showTitle: "",
              start: 100,
              end: 100000,
              durationMs: 99900,
            },
            next: { title: "Next film" },
            mode: "LIVE",
            positionMs: 10000,
            liveOffsetMs: 10000,
            durationMs: 99900,
            serverTime: 10100,
          },
        }),
      );
    }
    tune = mock.tune;
    goLive = mock.goLive;
    toggle = mock.toggle;
    retry = mock.retry;
    dispose = mock.dispose;
  },
}));
import { Player } from "./Player";
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.clearAllMocks();
  window.location.hash = "";
});
it("renders overlay and maps playback controls, channel keys and exit", async () => {
  vi.spyOn(playbackAPI, "onNow").mockResolvedValue({
    serverTime: 0,
    items: ["a", "b"].map((id, i) => ({
      channel: {
        id,
        number: String(101 + i),
        name: id,
        logo: "",
        recentKind: "",
      },
      program: null,
      next: null,
      progress: 0,
    })),
  });
  const exit = vi.fn();
  render(<Player channelId="a" csrf="csrf" onExit={exit} />);
  expect(await screen.findByText("Film")).toBeInTheDocument();
  expect(screen.getByText("NEXT · Next film")).toBeInTheDocument();
  expect(screen.getByText("● LIVE")).toBeInTheDocument();
  expect(mock.tune).toHaveBeenCalledWith("a");
  fireEvent.click(screen.getByText("Watch From Beginning"));
  expect(mock.tune).toHaveBeenCalledWith("a", "FROM_BEGINNING");
  fireEvent.click(screen.getByText("Go Live"));
  expect(mock.goLive).toHaveBeenCalled();
  fireEvent.keyDown(window, { key: "ArrowUp" });
  await waitFor(() => expect(window.location.hash).toBe("#watch/b"));
  window.location.hash = "";
  fireEvent.keyDown(window, { key: "ArrowDown" });
  expect(window.location.hash).toBe("#watch/b");
  fireEvent.keyDown(window, { key: " " });
  expect(mock.toggle).toHaveBeenCalled();
  fireEvent.keyDown(window, { key: "Escape" });
  expect(exit).toHaveBeenCalled();
});
