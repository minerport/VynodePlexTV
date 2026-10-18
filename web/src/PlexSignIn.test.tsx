import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { PlexSignIn } from "./PlexSignIn";
import { request } from "./api";
vi.mock("./api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./api")>()),
  request: vi.fn(),
}));
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
it("opens official Plex authorization without asking for a password or token", async () => {
  vi.mocked(request).mockResolvedValue({
    id: "link",
    authUrl: "https://app.plex.tv/auth#?code=fixture",
    expiresAt: 9999999999,
  });
  render(<PlexSignIn csrf="csrf" onConnected={vi.fn()} />);
  fireEvent.click(screen.getByRole("button", { name: "Sign in with Plex" }));
  const link = await screen.findByRole("link", { name: "Open Plex sign-in" });
  expect(link).toHaveAttribute(
    "href",
    "https://app.plex.tv/auth#?code=fixture",
  );
  expect(link).toHaveAttribute("rel", "noopener noreferrer");
  expect(screen.queryByLabelText(/password|token/i)).not.toBeInTheDocument();
});

it("selects automatic secure discovery without asking the user to pick a network interface", async () => {
  vi.mocked(request).mockImplementation(async (path) => {
    if (path.endsWith("/poll"))
      return {
        status: "AUTHORIZED",
        servers: [
          {
            id: "server",
            name: "Cinema",
            owned: true,
            connections: [
              {
                uri: "https://example.invalid",
                protocol: "https",
                local: true,
                relay: false,
              },
            ],
          },
        ],
      } as never;
    if (path.endsWith("/connect")) return { id: "saved" } as never;
    return {
      id: "link",
      authUrl: "https://app.plex.tv/auth",
      expiresAt: 9999999999,
    } as never;
  });
  const connected = vi.fn();
  render(<PlexSignIn csrf="csrf" onConnected={connected} />);
  fireEvent.click(screen.getByRole("button", { name: "Sign in with Plex" }));
  const connect = await screen.findByRole(
    "button",
    { name: "Connect this server" },
    { timeout: 4000 },
  );
  expect(screen.getByRole("combobox")).toHaveValue("");
  fireEvent.click(connect);
  expect(request).toHaveBeenCalledWith(
    "/plex/link/link/connect",
    "POST",
    { serverId: "server", uri: "" },
    "csrf",
  );
});
