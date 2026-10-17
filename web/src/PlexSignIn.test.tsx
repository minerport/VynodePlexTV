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
