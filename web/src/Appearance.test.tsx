import { beforeEach, expect, test } from "vitest";
import { fireEvent, render, screen, cleanup } from "@testing-library/react";
import {
  AppearanceSettings,
  readAppearance,
  applyAppearance,
} from "./Appearance";
beforeEach(() => {
  cleanup();
  localStorage.clear();
});
test("theme and density persist without changing programming", () => {
  render(<AppearanceSettings />);
  fireEvent.click(screen.getByRole("button", { name: /Ember/ }));
  expect(document.documentElement.dataset.theme).toBe("ember");
  fireEvent.click(screen.getByRole("button", { name: "Layout: Cinematic" }));
  expect(readAppearance()).toEqual({
    theme: "Ember",
    compact: true,
    effects: true,
  });
  fireEvent.click(screen.getByRole("button", { name: "Effects: Ambient" }));
  expect(document.documentElement.dataset.effects).toBe("off");
});
test("invalid stored theme falls back safely", () => {
  localStorage.setItem("afterglow.appearance", '{"theme":"unknown"}');
  applyAppearance(readAppearance());
  expect(document.documentElement.dataset.theme).toBe("tidal");
  localStorage.setItem("afterglow.appearance", "broken");
  expect(readAppearance().theme).toBe("Tidal");
});
