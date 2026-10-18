import { useEffect, useState } from "react";
import { request } from "../api";
interface Entry {
  id: string;
  title: string;
  startedAt: number;
  watchDurationMs: number;
  completionRatio: number;
  origin: string;
}
export function HistoryPrivacy({ csrf }: { csrf: string }) {
  const [items, setItems] = useState<Entry[]>([]),
    [offset, setOffset] = useState(0),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    void request<{ items: Entry[] }>(`/users/me/history?offset=${offset}`)
      .then((v) => {
        if (live) setItems(v.items);
      })
      .catch(() => {
        if (live) setMessage("Viewing history is unavailable.");
      });
    return () => {
      live = false;
    };
  }, [offset]);
  async function clear() {
    if (
      !window.confirm(
        "Clear your viewing history and inferred affinities? Favorites, channel order, hidden channels, Plex data and broadcast schedules will be kept.",
      )
    )
      return;
    setBusy(true);
    try {
      await request("/users/me/history", "DELETE", undefined, csrf);
      setItems([]);
      setOffset(0);
      setMessage("Your viewing history and inferred affinities were cleared.");
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Unable to clear history");
    } finally {
      setBusy(false);
    }
  }
  return (
    <section>
      <h2>Your viewing history</h2>
      <p>
        Private to your account. Retained for up to 180 days. Brief visits do
        not build strong affinities.
      </p>
      {message && <p role="status">{message}</p>}
      <button disabled={busy} onClick={() => void clear()}>
        Clear Viewing History
      </button>
      <button
        disabled={busy}
        onClick={() => {
          setBusy(true);
          void request("/personalization/rebuild", "POST", {}, csrf)
            .then(() => setMessage("Personalization rebuild queued."))
            .catch(() => setMessage("Unable to queue rebuild."))
            .finally(() => setBusy(false));
        }}
      >
        Rebuild my affinities
      </button>
      <ol>
        {items.map((h) => (
          <li key={h.id}>
            {h.title} · {Math.floor(h.watchDurationMs / 60000)} minutes watched
            · {Math.round(h.completionRatio * 100)}% ·{" "}
            {h.origin.replaceAll("_", " ")}
          </li>
        ))}
      </ol>
      {!items.length && <p>No viewing history on this page.</p>}
      <button
        disabled={offset === 0}
        onClick={() => setOffset(Math.max(0, offset - 50))}
      >
        Newer
      </button>
      <button
        disabled={items.length < 50}
        onClick={() => setOffset(offset + 50)}
      >
        Older
      </button>
    </section>
  );
}
