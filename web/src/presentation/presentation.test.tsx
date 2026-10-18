import { afterEach, expect, it, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { BroadcastOverlay, type BroadcastContext } from "./BroadcastOverlay";
import { PresentationAdmin } from "./PresentationAdmin";
import { request } from "../api";
import { presentationAPI, upload } from "./api";
vi.mock("../api", () => ({ request: vi.fn() }));
vi.mock("./api", async (importOriginal) => {
  const real = await importOriginal<typeof import("./api")>();
  return {
    ...real,
    upload: vi.fn(),
    presentationAPI: {
      assets: vi.fn(),
      rules: vi.fn(),
      asset: vi.fn(),
      rule: vi.fn(),
      branding: vi.fn(),
      saveBranding: vi.fn(),
      remove: vi.fn(),
    },
  };
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});
const branding = {
  channelId: "c",
  enabled: true,
  inherit: true,
  bug: true,
  corner: "top-right",
  opacity: 0.35,
  size: 72,
  compactLogoId: "",
  nextUpSeconds: 45,
};
const context: BroadcastContext = {
  channel: {
    id: "c",
    name: "Cinema",
    number: "101",
    logo: "/logo",
    recentKind: "",
  },
  program: { end: 60000 },
  next: {
    title: "Next movie",
    showTitle: "",
    season: null,
    episode: null,
    start: 60000,
  },
  origin: "LIVE",
  mode: "LIVE",
  state: "PLAYING",
  serverTime: 30000,
  delivery: "DIRECT_PLAY",
  branding,
};
it("shows restrained channel bug in each safe corner and next-up only near live boundary", () => {
  for (const corner of [
    "top-left",
    "top-right",
    "bottom-left",
    "bottom-right",
  ]) {
    const { unmount } = render(
      <BroadcastOverlay
        tune={{ ...context, branding: { ...branding, corner } }}
      />,
    );
    expect(screen.getByAltText("Cinema channel logo").className).toContain(
      corner,
    );
    expect(screen.getByLabelText("Coming Up Next")).toHaveTextContent(
      "Next movie",
    );
    unmount();
  }
});
it("suppresses next-up for catch-up, pause, behind-live and early program viewing", () => {
  for (const patch of [
    { origin: "CATCH_UP" as const },
    { state: "PAUSED" },
    { mode: "BEHIND_LIVE" as const },
    { serverTime: 0 },
  ]) {
    const { unmount } = render(
      <BroadcastOverlay tune={{ ...context, ...patch }} />,
    );
    expect(screen.queryByLabelText("Coming Up Next")).not.toBeInTheDocument();
    unmount();
  }
});
it("renders a scheduled metadata station break without video and hides bug over controls", () => {
  render(
    <BroadcastOverlay
      tune={{ ...context, delivery: "METADATA_PRESENTATION" }}
      controlsVisible
    />,
  );
  expect(screen.getByText("STATION BREAK")).toBeVisible();
  expect(screen.queryByAltText("Cinema channel logo")).not.toBeInTheDocument();
});
it("renders assets, upload validation, rule editor and non-mutating channel preview", async () => {
  vi.mocked(request).mockResolvedValue({
    items: [{ id: "c", name: "Cinema", number: "101" }],
    more: false,
  });
  vi.mocked(presentationAPI.assets).mockResolvedValue([
    {
      id: "a",
      name: "Ident",
      kind: "IDENT",
      source: "UPLOAD",
      durationMs: 5000,
      mediaId: "",
      enabled: true,
      available: true,
      tags: [],
      activeFrom: "",
      activeUntil: "",
    },
  ]);
  vi.mocked(presentationAPI.rules).mockResolvedValue([]);
  vi.mocked(presentationAPI.branding).mockResolvedValue(branding);
  render(
    <PresentationAdmin
      session={{
        user: { id: "u", username: "Owner", role: "OWNER" },
        csrfToken: "test",
        expiresAt: 1,
      }}
    />,
  );
  await screen.findByText("Ident");
  fireEvent.click(screen.getByText("Preview clip"));
  expect(document.querySelector("video")?.getAttribute("src")).toContain(
    "/a/preview",
  );
  fireEvent.change(screen.getByLabelText("Clip name"), {
    target: { value: "Test" },
  });
  fireEvent.submit(screen.getByText("Upload clip").closest("form")!);
  expect(await screen.findByRole("alert")).toHaveTextContent("Choose an MP4");
  expect(upload).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("Choose channel"), {
    target: { value: "c" },
  });
  await screen.findByLabelText("Branding preview");
  expect(presentationAPI.saveBranding).not.toHaveBeenCalled();
  fireEvent.click(screen.getByText("Save channel presentation"));
  await waitFor(() => expect(presentationAPI.saveBranding).toHaveBeenCalled());
  fireEvent.change(screen.getByLabelText("Rule name"), {
    target: { value: "Between episodes" },
  });
  fireEvent.submit(screen.getByText("Save rule").closest("form")!);
  await waitFor(() =>
    expect(presentationAPI.rule).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Between episodes" }),
      "test",
    ),
  );
  fireEvent.click(screen.getByText("Disable"));
  await waitFor(() =>
    expect(presentationAPI.asset).toHaveBeenCalledWith(
      expect.objectContaining({ id: "a", enabled: false }),
      "test",
    ),
  );
});
