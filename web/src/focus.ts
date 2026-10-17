import { useEffect } from "react";

export function moveFocus(root: HTMLElement, key: string): boolean {
  const active = document.activeElement as HTMLElement | null;
  if (active?.matches('input, textarea, select, [contenteditable="true"]'))
    return false;
  const elements = Array.from(
    root.querySelectorAll<HTMLElement>(
      'a[href],button:not([disabled]),input:not([disabled]),textarea:not([disabled]),select:not([disabled]),[tabindex="0"]',
    ),
  ).filter((el) => !el.closest("[hidden]") && el.getClientRects().length > 0);
  if (!elements.length) return false;
  const index = active ? elements.indexOf(active) : -1;
  const forward = key === "ArrowRight" || key === "ArrowDown";
  const next =
    elements[
      index < 0
        ? 0
        : (index + (forward ? 1 : -1) + elements.length) % elements.length
    ];
  next.focus();
  next.scrollIntoView({
    block: "nearest",
    inline: "nearest",
    behavior: "smooth",
  });
  return true;
}
export function useTVFocus(onBack: () => void) {
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      if (event.key === "Escape") {
        event.preventDefault();
        onBack();
        return;
      }
      if (
        ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight"].includes(
          event.key,
        ) &&
        moveFocus(document.body, event.key)
      )
        event.preventDefault();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [onBack]);
}
