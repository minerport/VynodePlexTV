import { guideAPI } from "./api";
import { useEffect, useRef, useState, type ReactNode } from "react";
export function Dialog({
  label,
  onClose,
  children,
  className = "",
}: {
  label: string;
  onClose: () => void;
  children: ReactNode;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const prior = document.activeElement as HTMLElement;
    ref.current?.querySelector<HTMLElement>("button")?.focus();
    return () => {
      if (prior?.isConnected) prior.focus();
    };
  }, []);
  return (
    <div className="dialog-scrim">
      <div
        className={"tv-dialog " + className}
        role="dialog"
        aria-modal="true"
        aria-label={label}
        ref={ref}
        onKeyDown={(e) => {
          e.stopPropagation();
          if (e.key === "Escape") {
            e.preventDefault();
            onClose();
          }
          if (e.key === "Tab") {
            const all = Array.from(
              ref.current?.querySelectorAll<HTMLElement>(
                'button:not(:disabled),a,input,select,[tabindex="0"]',
              ) ?? [],
            );
            const tabbable = all.filter((el) => el.tabIndex >= 0);
            const i = tabbable.indexOf(document.activeElement as HTMLElement);
            if (e.shiftKey && i <= 0) {
              e.preventDefault();
              tabbable.at(-1)?.focus();
            } else if (!e.shiftKey && i === tabbable.length - 1) {
              e.preventDefault();
              tabbable[0]?.focus();
            }
          }
          if (
            ["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(
              e.key,
            ) &&
            !(e.target instanceof HTMLInputElement) &&
            !(e.target as HTMLElement).closest("[data-spatial]")
          ) {
            e.preventDefault();
            const all = Array.from(
              ref.current?.querySelectorAll<HTMLButtonElement>(
                "button:not(:disabled)",
              ) ?? [],
            );
            const i = all.indexOf(document.activeElement as HTMLButtonElement);
            all[
              (i +
                (["ArrowRight", "ArrowDown"].includes(e.key) ? 1 : -1) +
                all.length) %
                all.length
            ]?.focus();
          }
        }}
      >
        <button className="dialog-close" onClick={onClose}>
          ← Back
        </button>
        {children}
      </div>
    </div>
  );
}
export function FavoriteButton({
  id,
  favorite,
  csrf,
  onChange,
}: {
  id: string;
  favorite: boolean;
  csrf: string;
  onChange?: (on: boolean) => void;
}) {
  const [on, setOn] = useState(favorite),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [previous, setPrevious] = useState(favorite);
  if (previous !== favorite) {
    setPrevious(favorite);
    setOn(favorite);
  }
  return (
    <>
      <button
        aria-label={on ? "★ Unfavorite Channel" : "☆ Favorite Channel"}
        aria-pressed={on}
        disabled={busy}
        onClick={async () => {
          setBusy(true);
          try {
            await guideAPI.favorite(id, !on, csrf);
            setOn(!on);
            onChange?.(!on);
            setError("");
          } catch {
            setError("Favorite could not be saved. Try again.");
          } finally {
            setBusy(false);
          }
        }}
      >
        {on ? "★ Unfavorite Channel" : "☆ Favorite Channel"}
      </button>
      {error && <span role="alert">{error}</span>}
    </>
  );
}
