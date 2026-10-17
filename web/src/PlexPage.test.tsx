import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PlexPage, JobProgress } from "./PlexPage";
import {
  plexAPI,
  type PlexJob,
  type PlexServer,
  type PlexLibrary,
} from "./plex-api";
import type { Session } from "./api";

const session: Session = {
  user: { id: "owner", username: "owner", role: "OWNER" },
  csrfToken: "test-csrf",
  expiresAt: 9999999999,
};
const server: PlexServer = {
  id: "s",
  name: "Cinema",
  baseUrl: "http://plex.test",
  machineIdentifier: "machine",
  friendlyName: "Cinema",
  version: "1",
  platform: "Linux",
  status: "OFFLINE",
  lastSuccessAt: 1,
  lastHealthCheckAt: 2,
  lastError: "Cannot reach Plex",
  disabled: false,
};
const library: PlexLibrary = {
  id: "l",
  serverId: "s",
  key: "1",
  title: "Movies",
  type: "movie",
  selected: false,
  present: true,
  supported: true,
  lastSyncedAt: null,
  itemCount: 0,
};
beforeEach(() => {
  vi.spyOn(plexAPI, "servers").mockResolvedValue([]);
  vi.spyOn(plexAPI, "jobs").mockResolvedValue([]);
  vi.spyOn(plexAPI, "libraries").mockResolvedValue([]);
  vi.spyOn(plexAPI, "action").mockResolvedValue({} as PlexJob);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
it("adds a server and never renders the token as text", async () => {
  const token = crypto.randomUUID();
  const save = vi
    .spyOn(plexAPI, "save")
    .mockResolvedValue({ ...server, status: "CONNECTED" });
  render(<PlexPage session={session} />);
  await screen.findByRole("button", { name: "Add Plex server" });
  fireEvent.click(screen.getByRole("button", { name: "Add Plex server" }));
  fireEvent.change(screen.getByLabelText("Server URL"), {
    target: { value: "http://plex.test" },
  });
  fireEvent.change(screen.getByLabelText("Plex token"), {
    target: { value: token },
  });
  expect(screen.getByLabelText("Plex token")).toHaveAttribute(
    "type",
    "password",
  );
  expect(screen.queryByText(token)).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Connect server" }));
  await waitFor(() =>
    expect(save).toHaveBeenCalledWith(
      { baseUrl: "http://plex.test", token, name: "" },
      undefined,
      "test-csrf",
    ),
  );
  await waitFor(() =>
    expect(screen.queryByLabelText("Plex token")).not.toBeInTheDocument(),
  );
  expect(window.localStorage.length).toBe(0);
});
it.each(["Plex rejected the token", "Plex returned an unexpected response"])(
  "shows connection failure: %s",
  async (message) => {
    vi.spyOn(plexAPI, "test").mockRejectedValue(new Error(message));
    render(<PlexPage session={session} />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Add Plex server" }),
    );
    fireEvent.change(screen.getByLabelText("Server URL"), {
      target: { value: "http://plex.test" },
    });
    fireEvent.change(screen.getByLabelText("Plex token"), {
      target: { value: crypto.randomUUID() },
    });
    fireEvent.click(screen.getByRole("button", { name: "Test connection" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
  },
);
it("displays offline status and selects and synchronizes a library", async () => {
  vi.mocked(plexAPI.servers).mockResolvedValue([server]);
  vi.mocked(plexAPI.libraries).mockResolvedValue([library]);
  const select = vi.spyOn(plexAPI, "select").mockImplementation(async () => {
    vi.mocked(plexAPI.libraries).mockResolvedValue([
      { ...library, selected: true },
    ]);
  });
  const sync = vi.spyOn(plexAPI, "sync").mockResolvedValue({} as PlexJob);
  render(<PlexPage session={session} />);
  expect(await screen.findByText("OFFLINE")).toBeInTheDocument();
  expect(screen.getByText("Cannot reach Plex")).toBeInTheDocument();
  fireEvent.click(
    within(screen.getByRole("navigation", { name: "Plex sections" })).getByRole(
      "button",
      { name: "Libraries" },
    ),
  );
  await screen.findByRole("heading", { name: "Movies" });
  fireEvent.click(screen.getByRole("button", { name: "Select library" }));
  await waitFor(() =>
    expect(select).toHaveBeenCalledWith("l", true, "test-csrf"),
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Sync changes" })).toBeEnabled(),
  );
  fireEvent.click(screen.getByRole("button", { name: "Sync changes" }));
  await waitFor(() =>
    expect(sync).toHaveBeenCalledWith("l", "incremental", "test-csrf"),
  );
});
it("shows honest determinate and indeterminate job progress", () => {
  const job: PlexJob = {
    id: "j",
    type: "PLEX_LIBRARY_SYNC",
    serverId: "s",
    libraryId: "l",
    mode: "full",
    status: "RUNNING",
    processed: 5,
    total: 10,
    currentItem: "A movie",
    error: "",
    createdAt: 1,
    completedAt: null,
    cancelRequested: false,
  };
  const { rerender } = render(<JobProgress job={job} />);
  expect(screen.getByRole("progressbar")).toHaveAttribute("value", "5");
  expect(screen.getByText(/50%/)).toBeInTheDocument();
  rerender(<JobProgress job={{ ...job, total: null }} />);
  expect(screen.getByRole("progressbar")).not.toHaveAttribute("value");
  expect(screen.queryByText(/50%/)).not.toBeInTheDocument();
});
it("browses real API metadata and availability", async () => {
  vi.spyOn(plexAPI, "media").mockResolvedValue({
    items: [
      {
        id: "i",
        serverId: "s",
        libraryId: "l",
        libraryTitle: "Movies",
        ratingKey: "10",
        parentRatingKey: "",
        grandparentRatingKey: "",
        title: "Local inventory movie",
        type: "movie",
        year: 2024,
        summary: "",
        available: false,
        updatedAt: 1,
        index: null,
        parentIndex: null,
        contentRating: "PG",
        rating: 8,
      },
    ],
    total: 1,
    offset: 0,
    limit: 24,
  });
  render(<PlexPage session={session} />);
  await screen.findByRole("button", { name: "Add Plex server" });
  fireEvent.click(screen.getByRole("button", { name: "Media browser" }));
  expect(await screen.findByText("Local inventory movie")).toBeInTheDocument();
  expect(
    screen.getByText("Unavailable", { selector: "span" }),
  ).toBeInTheDocument();
});
it("does not load administration for a USER", () => {
  render(
    <PlexPage
      session={{ ...session, user: { ...session.user, role: "USER" } }}
    />,
  );
  expect(screen.getByText(/Your administrator manages/)).toBeInTheDocument();
  expect(plexAPI.servers).not.toHaveBeenCalled();
});
it("groups TV into shows, seasons and episodes with working breadcrumbs", async () => {
  const common = {
    serverId: "s",
    libraryId: "l",
    libraryTitle: "TV",
    year: 2024,
    summary: "",
    available: true,
    updatedAt: 1,
    contentRating: "TV-PG",
    rating: 8,
    index: null,
    parentIndex: null,
    parentRatingKey: "",
    grandparentRatingKey: "",
  };
  const show = {
    ...common,
    id: "show",
    ratingKey: "20",
    title: "Night Show",
    type: "show",
  };
  const season = {
    ...common,
    id: "season",
    ratingKey: "21",
    parentRatingKey: "20",
    title: "Season 1",
    type: "season",
  };
  const episode = {
    ...common,
    id: "episode",
    ratingKey: "22",
    parentRatingKey: "21",
    grandparentRatingKey: "20",
    title: "Pilot",
    type: "episode",
  };
  const media = vi
    .spyOn(plexAPI, "media")
    .mockImplementation(async (params) => {
      const item =
        params.get("parent") === "21"
          ? episode
          : params.get("parent") === "20"
            ? season
            : show;
      return { items: [item], total: 1, offset: 0, limit: 24 };
    });
  vi.spyOn(plexAPI, "item").mockResolvedValue(episode);
  render(<PlexPage session={session} />);
  await screen.findByRole("button", { name: "Add Plex server" });
  fireEvent.click(screen.getByRole("button", { name: "Media browser" }));
  fireEvent.click(
    await screen.findByRole("button", { name: /Night Show.*Available/ }),
  );
  expect(media.mock.calls[0][0].get("topLevel")).toBe("true");
  fireEvent.click(
    await screen.findByRole("button", { name: /Season 1.*Available/ }),
  );
  fireEvent.click(
    await screen.findByRole("button", { name: /Pilot.*Available/ }),
  );
  expect(
    await screen.findByRole("region", { name: "Media details" }),
  ).toBeInTheDocument();
  const crumbs = screen.getByRole("navigation", {
    name: "TV library hierarchy",
  });
  fireEvent.click(within(crumbs).getByRole("button", { name: "Night Show" }));
  expect(
    await screen.findByRole("button", { name: /Season 1.*Available/ }),
  ).toBeInTheDocument();
  fireEvent.click(
    within(crumbs).getByRole("button", { name: "Movies & shows" }),
  );
  expect(
    await screen.findByRole("button", { name: /Night Show.*Available/ }),
  ).toBeInTheDocument();
  expect(
    screen.queryByRole("navigation", { name: "TV library hierarchy" }),
  ).not.toBeInTheDocument();
});
