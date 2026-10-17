import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { App } from "./App";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
it("shows first-owner setup without pretending Plex is connected", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ setupRequired: true }),
    }),
  );
  render(<App />);
  expect(
    await screen.findByRole("button", { name: /Create owner account/ }),
  ).toBeInTheDocument();
  expect(screen.getByText(/Connect Plex now/)).toBeInTheDocument();
});
it("shows a recoverable offline state", async () => {
  vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
  render(<App />);
  expect(
    await screen.findByRole("button", { name: "Try again" }),
  ).toBeInTheDocument();
  expect(screen.getByRole("alert")).toHaveTextContent("Cannot reach Afterglow");
});
