import { useEffect, useState } from "react";
import { request } from "../api";
import { guideAPI } from "../guide/api";
import "./personalization.css";
import { HistoryPrivacy } from "./HistoryPrivacy";

interface Channel {
  id: string;
  number: string;
  name: string;
  hidden: boolean;
  favorite: boolean;
}
interface Preferences {
  mode: "NUMBER" | "PERSONAL";
  channels: Channel[];
}
export function MyChannels({ csrf }: { csrf: string }) {
  const [data, setData] = useState<Preferences | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [drag, setDrag] = useState<string | null>(null);
  const load = () => request<Preferences>("/users/me/channels").then(setData);
  useEffect(() => {
    void load().catch((e) => setError(e.message));
  }, []);
  async function change(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to save preferences");
    } finally {
      setBusy(false);
    }
  }
  function move(id: string, target: string) {
    if (!data || id === target) return;
    const ids = data.channels.map((c) => c.id);
    const from = ids.indexOf(id),
      to = ids.indexOf(target);
    if (from < 0 || to < 0) return;
    ids.splice(from, 1);
    ids.splice(to, 0, id);
    void change(() =>
      request(
        "/users/me/channels/order",
        "PATCH",
        { mode: "PERSONAL", channelIds: ids },
        csrf,
      ),
    );
  }
  return (
    <section className="my-channels">
      <p className="eyebrow">YOUR TELEVISION</p>
      <h1>My Channels</h1>
      <p>
        Arrange your guide. Channel numbers and shared programming stay the
        same.
      </p>
      {error && <p role="alert">{error}</p>}
      {!data ? (
        <p role="status">Loading your channels…</p>
      ) : (
        <>
          <div className="personal-toolbar">
            <label>
              Browse in{" "}
              <select
                disabled={busy}
                value={data.mode}
                onChange={(e) =>
                  void change(() =>
                    request(
                      "/users/me/channels/order",
                      "PATCH",
                      { mode: e.target.value },
                      csrf,
                    ),
                  )
                }
              >
                <option value="NUMBER">Channel Number Order</option>
                <option value="PERSONAL">My Order</option>
              </select>
            </label>
            <button
              disabled={busy}
              onClick={() => {
                if (
                  window.confirm(
                    "Reset your channel order and show all hidden channels? Favorites will be kept.",
                  )
                )
                  void change(() =>
                    request("/users/me/channels/reset", "POST", {}, csrf),
                  );
              }}
            >
              Reset order and visibility
            </button>
          </div>
          <p className="muted">
            Drag to reorder, or use Move up / Move down. Hidden channels can
            still be tuned by their exact number.
          </p>
          {!data.channels.length ? (
            <p>No channels available yet.</p>
          ) : (
            <ol className="personal-channel-list">
              {data.channels.map((c, i) => (
                <li
                  key={c.id}
                  draggable={!busy}
                  onDragStart={() => setDrag(c.id)}
                  onDragEnd={() => setDrag(null)}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={(e) => {
                    e.preventDefault();
                    if (drag) move(drag, c.id);
                    setDrag(null);
                  }}
                >
                  <div>
                    <strong>
                      {c.number} · {c.name}
                    </strong>
                    <small>
                      {c.hidden
                        ? "Hidden from your browsing"
                        : "Visible in your guide"}
                    </small>
                  </div>
                  <button
                    disabled={busy}
                    aria-pressed={c.favorite}
                    onClick={() =>
                      void change(() =>
                        guideAPI.favorite(c.id, !c.favorite, csrf),
                      )
                    }
                  >
                    {c.favorite ? "★ Favorite" : "☆ Favorite"}
                  </button>
                  <button
                    disabled={busy}
                    aria-label={`${c.hidden ? "Show" : "Hide"} ${c.name}`}
                    onClick={() =>
                      void change(() =>
                        request(
                          `/users/me/channels/${c.id}/hide`,
                          c.hidden ? "DELETE" : "POST",
                          undefined,
                          csrf,
                        ),
                      )
                    }
                  >
                    {c.hidden ? "Show" : "Hide"}
                  </button>
                  <button
                    disabled={busy || i === 0}
                    aria-label={`Move ${c.name} up`}
                    onClick={() => move(c.id, data.channels[i - 1].id)}
                  >
                    ↑
                  </button>
                  <button
                    disabled={busy || i === data.channels.length - 1}
                    aria-label={`Move ${c.name} down`}
                    onClick={() => move(c.id, data.channels[i + 1].id)}
                  >
                    ↓
                  </button>
                </li>
              ))}
            </ol>
          )}
        </>
      )}
      <HistoryPrivacy csrf={csrf} />
    </section>
  );
}
