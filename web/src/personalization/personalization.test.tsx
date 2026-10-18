import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
} from "@testing-library/react";
import { request } from "../api";
import { MyChannels } from "./MyChannels";
import { ForYou } from "./ForYou";
import { SmartChannels } from "./SmartChannels";
import { channelAPI } from "../channels/api";
vi.mock("../api", () => ({ request: vi.fn() }));
vi.mock("../channels/api", () => ({ channelAPI: { metadata: vi.fn() } }));
beforeEach(() => {
  vi.mocked(request).mockImplementation(async (path) =>
    path.startsWith("/users/me/history")
      ? { items: [] }
      : path === "/settings"
        ? { timezone: "UTC" }
        : {
            mode: "NUMBER",
            channels: [
              {
                id: "a",
                number: "101",
                name: "Cinema",
                hidden: false,
                favorite: false,
              },
              {
                id: "b",
                number: "201",
                name: "Comedy",
                hidden: true,
                favorite: true,
              },
            ],
          },
  );
});
afterEach(() => {
  cleanup();
  vi.resetAllMocks();
});

it.each([
  ["director", "director"],
  ["decade", "year"],
  ["seasonal", "label"],
])("previews %s template with explicit options", async (template, field) => {
  vi.mocked(request).mockImplementation(async (path) =>
    path === "/smart-channel-templates"
      ? {
          items: [
            {
              id: template,
              name: "Template",
              description: "Explicit rules",
              field,
            },
          ],
        }
      : path === "/settings"
        ? { timezone: "UTC" }
        : { preview: { count: 7, items: [] } },
  );
  vi.mocked(channelAPI.metadata).mockResolvedValue({
    items: [
      {
        id: "3",
        name: "Synthetic choice",
        context: "Local",
        count: 7,
        artworkId: "",
        seasons: 0,
        episodes: 0,
      },
    ],
    more: false,
  });
  render(
    <SmartChannels csrf="fixture" onClose={() => {}} onCreated={() => {}} />,
  );
  fireEvent.click(await screen.findByRole("button", { name: "Template" }));
  if (field !== "year") {
    await screen.findByRole("option", { name: "Synthetic choice · Local" });
    fireEvent.change(screen.getByLabelText("Choose " + field), {
      target: { value: "3" },
    });
  }
  if (template === "seasonal") {
    fireEvent.change(screen.getByLabelText("First active date"), {
      target: { value: "2026-10-01" },
    });
    fireEvent.change(screen.getByLabelText("Last active date"), {
      target: { value: "2026-10-31" },
    });
  }
  fireEvent.change(screen.getByLabelText("Programming order"), {
    target: { value: "SEQUENTIAL" },
  });
  fireEvent.change(screen.getByLabelText("Repeat window (days)"), {
    target: { value: "14" },
  });
  fireEvent.change(screen.getByLabelText("Minimum rating"), {
    target: { value: "7" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Preview membership" }));
  expect(await screen.findByText("7 matching items")).toBeInTheDocument();
  expect(request).toHaveBeenCalledWith(
    "/smart-channel-templates/preview",
    "POST",
    expect.objectContaining({
      template,
      scheduleMode: "SEQUENTIAL",
      repeatDays: 14,
      minimumRating: 7,
    }),
    "fixture",
  );
});
it("reorders with keyboard accessible buttons without renumbering", async () => {
  render(<MyChannels csrf="fixture" />);
  fireEvent.click(
    await screen.findByRole("button", { name: "Move Comedy up" }),
  );
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/users/me/channels/order",
      "PATCH",
      { mode: "PERSONAL", channelIds: ["b", "a"] },
      "fixture",
    ),
  );
  expect(screen.getByText("101 · Cinema")).toBeInTheDocument();
});
it("shows and hides only personal channel preferences", async () => {
  render(<MyChannels csrf="fixture" />);
  fireEvent.click(await screen.findByRole("button", { name: "Show Comedy" }));
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/users/me/channels/b/hide",
      "DELETE",
      undefined,
      "fixture",
    ),
  );
});
it("supports drag reorder and reset confirmation", async () => {
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<MyChannels csrf="fixture" />);
  const a = (await screen.findByText("101 · Cinema")).closest("li")!,
    b = screen.getByText("201 · Comedy").closest("li")!;
  fireEvent.dragStart(b);
  fireEvent.dragOver(a);
  fireEvent.drop(a);
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/users/me/channels/order",
      "PATCH",
      { mode: "PERSONAL", channelIds: ["b", "a"] },
      "fixture",
    ),
  );
  await waitFor(() =>
    expect(
      screen.getByRole("button", { name: "Reset order and visibility" }),
    ).not.toBeDisabled(),
  );
  fireEvent.click(
    screen.getByRole("button", { name: "Reset order and visibility" }),
  );
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/users/me/channels/reset",
      "POST",
      {},
      "fixture",
    ),
  );
});
it("clears only personal history with an explicit scope", async () => {
  vi.spyOn(window, "confirm").mockReturnValue(true);
  render(<MyChannels csrf="fixture" />);
  fireEvent.click(
    screen.getByRole("button", { name: "Clear Viewing History" }),
  );
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      "/users/me/history",
      "DELETE",
      undefined,
      "fixture",
    ),
  );
  expect(window.confirm).toHaveBeenCalledWith(
    expect.stringContaining("broadcast schedules will be kept"),
  );
});
it("explains cold-start ranking and keeps future items non-tunable", async () => {
  vi.mocked(request).mockResolvedValue({
    coldStart: true,
    items: [
      {
        channel: { id: "a", number: "101", name: "Cinema" },
        program: {
          id: "p",
          title: "Film",
          state: "FUTURE",
          progress: 0,
          artwork: "/safe",
        },
        reasons: ["Favorite channel"],
      },
    ],
  });
  render(<ForYou csrf="fixture" expanded />);
  expect(await screen.findByText("Favorite channel")).toBeInTheDocument();
  expect(
    screen.queryByRole("button", { name: "Watch Live" }),
  ).not.toBeInTheDocument();
  expect(screen.getByText(/After a few meaningful visits/)).toBeInTheDocument();
});
it("previews explicit actor selections before creating a shared channel", async () => {
  vi.mocked(request).mockImplementation(async (path) =>
    path === "/smart-channel-templates"
      ? {
          items: [
            {
              id: "actor",
              name: "Actor Network",
              description: "Choose an actor",
              field: "actor",
            },
          ],
        }
      : path === "/settings"
        ? { timezone: "UTC" }
        : { preview: { count: 12, items: [] } },
  );
  vi.mocked(channelAPI.metadata).mockResolvedValue({
    items: [
      {
        id: "2",
        name: "Synthetic Actor",
        context: "Local",
        count: 12,
        artworkId: "",
        seasons: 0,
        episodes: 0,
      },
    ],
    more: false,
  });
  render(
    <SmartChannels csrf="fixture" onClose={() => {}} onCreated={() => {}} />,
  );
  fireEvent.click(await screen.findByRole("button", { name: "Actor Network" }));
  await screen.findByRole("option", { name: "Synthetic Actor · Local" });
  fireEvent.change(screen.getByLabelText("Choose actor"), {
    target: { value: "2" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Preview membership" }));
  expect(await screen.findByText("12 matching items")).toBeInTheDocument();
  expect(request).toHaveBeenCalledWith(
    "/smart-channel-templates/preview",
    "POST",
    expect.objectContaining({ template: "actor", selection: "2" }),
    "fixture",
  );
});
