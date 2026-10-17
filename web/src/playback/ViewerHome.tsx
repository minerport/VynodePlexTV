import { useEffect, useState, type ReactNode } from "react";
import { request } from "../api";
import { playbackAPI, watch, type OnNow } from "./api";
import "./player.css";
export function ViewerHome({
  fallback,
  userId,
}: {
  fallback: ReactNode;
  userId: string;
}) {
  const [items, setItems] = useState<OnNow[] | null>(null);
  const [error, setError] = useState("");
  const last = localStorage.getItem(`afterglow-last-channel-${userId}`);
  useEffect(() => {
    let alive = true;
    const load = () =>
      playbackAPI
        .onNow()
        .then((v) => {
          if (alive) {
            setItems(v.items);
            setError("");
          }
        })
        .catch(() => {
          if (alive)
            setError("Channel information is temporarily unavailable.");
        });
    void load();
    const t = setInterval(() => void load(), 30000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);
  if (!items?.length)
    return (
      <>
        {error && <p role="status">{error}</p>}
        {fallback}
      </>
    );
  const tune = (id: string) => {
    localStorage.setItem(`afterglow-last-channel-${userId}`, id);
    watch(id);
  };
  const cards = (list: OnNow[]) => (
    <div className="on-now-grid">
      {list.map((v) => (
        <article className="on-now-card" key={v.channel.id}>
          <img src={v.channel.logo} alt="" loading="lazy" />
          <span className="eyebrow">
            {v.channel.number} · {v.channel.name}
          </span>
          <h3>
            {v.program?.showTitle || v.program?.title || "Schedule not ready"}
          </h3>
          {v.program?.showTitle && (
            <p>
              {v.program.title} · S{v.program.season} E{v.program.episode}
            </p>
          )}
          <progress max="100" value={v.progress} />
          <p>Next · {v.next?.title ?? "Not scheduled"}</p>
          <button
            className="primary"
            disabled={!v.program}
            onClick={() => tune(v.channel.id)}
          >
            Watch Live
          </button>
        </article>
      ))}
    </div>
  );
  return (
    <section className="viewer-home">
      <span className="eyebrow">YOUR PERSONAL TELEVISION</span>
      <h1>
        Something good.
        <br />
        <em>Already on.</em>
      </h1>
      <p>Pick a channel. Settle into the moment.</p>
      {items.some((v) => v.channel.id === last) && (
        <button className="primary" onClick={() => tune(last!)}>
          Watch Last Channel
        </button>
      )}
      {error && <p role="status">{error}</p>}
      <h2 className="on-now-heading">On Now</h2>
      {cards(items.filter((v) => !v.channel.recentKind))}
      {items.some((v) => v.channel.recentKind) && (
        <>
          <h2 className="on-now-heading">Fresh in your collection</h2>
          {cards(items.filter((v) => !!v.channel.recentKind))}
        </>
      )}
    </section>
  );
}
interface Active {
  id: string;
  user: string;
  channel: string;
  program: string;
  client: string;
  mode: string;
  state: string;
  delivery: string;
  started: number;
  lastSeen: number;
}
export function ActiveViewers() {
  const [rows, setRows] = useState<Active[]>([]);
  const [error, setError] = useState("");
  useEffect(() => {
    let alive = true;
    const load = () =>
      request<Active[]>("/playback/active")
        .then((v) => {
          if (alive) {
            setRows(v);
            setError("");
          }
        })
        .catch(() => {
          if (alive) setError("Active viewers could not be loaded");
        });
    void load();
    const t = setInterval(() => void load(), 10000);
    return () => {
      alive = false;
      clearInterval(t);
    };
  }, []);
  return (
    <section className="active-viewers">
      <h2>On the air</h2>
      <p>Active application viewers</p>
      {error ? (
        <p role="status">{error}</p>
      ) : rows.length ? (
        <ul>
          {rows.map((v) => (
            <li key={v.id}>
              <strong>{v.user}</strong> · {v.channel} · {v.program}
              <br />
              <small>
                {v.client} · {v.mode} · {v.delivery} · {v.state} · Last seen{" "}
                {new Date(v.lastSeen).toLocaleTimeString()}
              </small>
            </li>
          ))}
        </ul>
      ) : (
        <p>No active viewers.</p>
      )}
    </section>
  );
}
