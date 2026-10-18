import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { AudioControls } from "./AudioControls";
it("changes volume and mutes without restarting media", () => {
  const video = document.createElement("video");
  const load = vi.spyOn(video, "load");
  render(<AudioControls getVideo={() => video} />);
  fireEvent.change(screen.getByRole("slider", { name: "Volume" }), {
    target: { value: "50" },
  });
  expect(video.volume).toBe(0.5);
  fireEvent.click(screen.getByText("Mute"));
  expect(video.muted).toBe(true);
  fireEvent.click(screen.getByText("Unmute"));
  expect(video.volume).toBe(1);
  expect(video.muted).toBe(false);
  expect(load).not.toHaveBeenCalled();
});

it("boosts through one audio graph and releases it on exit", async () => {
  const gain = { gain: { value: 1 }, connect: vi.fn() };
  const close = vi.fn().mockResolvedValue(undefined);
  const source = vi.fn(() => ({ connect: vi.fn() }));
  class Context {
    destination = {};
    createMediaElementSource = source;
    createGain = () => gain;
    resume = vi.fn().mockResolvedValue(undefined);
    close = close;
  }
  vi.stubGlobal("AudioContext", Context);
  const video = document.createElement("video");
  const { unmount } = render(<AudioControls getVideo={() => video} />);
  fireEvent.click(screen.getByText("Boost audio"));
  await vi.waitFor(() => expect(gain.gain.value).toBe(1.5));
  fireEvent.click(screen.getByText("Reset boost"));
  await vi.waitFor(() => expect(gain.gain.value).toBe(1));
  expect(source).toHaveBeenCalledTimes(1);
  unmount();
  expect(close).toHaveBeenCalledTimes(1);
  vi.unstubAllGlobals();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
