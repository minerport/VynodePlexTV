import { afterEach, describe, expect, it, vi } from "vitest";
import { moveFocus } from "./focus";

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
});
describe("TV focus", () => {
  it("moves deterministically, wraps and scrolls focused controls into view", () => {
    document.body.innerHTML =
      "<button>A</button><button>B</button><button disabled>C</button>";
    vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([
      { width: 10 },
    ] as unknown as DOMRectList);
    HTMLElement.prototype.scrollIntoView = vi.fn();
    const buttons = document.querySelectorAll("button");
    buttons[0].focus();
    expect(moveFocus(document.body, "ArrowRight")).toBe(true);
    expect(document.activeElement).toBe(buttons[1]);
    moveFocus(document.body, "ArrowDown");
    expect(document.activeElement).toBe(buttons[0]);
    moveFocus(document.body, "ArrowLeft");
    expect(document.activeElement).toBe(buttons[1]);
    expect(buttons[1].scrollIntoView).toHaveBeenCalled();
  });
  it("preserves native input editing and handles empty screens", () => {
    expect(moveFocus(document.body, "ArrowDown")).toBe(false);
    document.body.innerHTML = "<input/><button>Save</button>";
    const input = document.querySelector("input")!;
    input.focus();
    expect(moveFocus(document.body, "ArrowLeft")).toBe(false);
    expect(document.activeElement).toBe(input);
  });
});
