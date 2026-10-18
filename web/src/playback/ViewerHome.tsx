import { chooseGuideGroup } from "../guide/GuideGrid";
import { ForYou } from "../personalization/ForYou";
import { guideAPI, type WindowData, type Program } from "../guide/api";
import { ProgramDetails } from "../guide/ProgramDetails";
import { FavoriteButton } from "../guide/Dialog";
import { timeLabel } from "../guide/timeline";
import "../guide/guide.css";
import { useEffect, useState, type ReactNode } from "react";
import { request } from "../api";
import { playbackAPI, watch, type OnNow } from "./api";
import "./player.css";
export function ViewerHome({
  fallback,
  userId,
  csrf = "",
}: {
  fallback: ReactNode;
  userId: string;
  csrf?: string;
}) {
  const [items, setItems] = useState<OnNow[] | null>(null);
  const [error, setError] = useState("");
  const [last, setLast] = useState(
    localStorage.getItem(`afterglow-last-channel-${userId}`),
  );
  const [favorites, setFavorites] = useState<string[]>([]),
    [soon, setSoon] = useState<WindowData | null>(null),
    [details, setDetails] = useState<string | null>(null);
  const refreshFavorites = () =>
    guideAPI.favorites().then((v) => setFavorites(v.channelIds));
  useEffect(() => {
    void guideAPI
      .last()
      .then((v) => setLast(v.channelId))
      .catch(() => {});
    void refreshFavorites().catch(() => {});
    const load = () =>
      guideAPI
        .soon()
        .then(setSoon)
        .catch(() => {});
    void load();
    const t = setInterval(() => void load(), 30000);
    return () => clearInterval(t);
  }, []);
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
          {v.program?.artwork && (
            <img
              className="program-art"
              src={v.program.artwork}
              alt=""
              loading="lazy"
            />
          )}
          <img
            className="channel-badge"
            src={v.channel.logo}
            alt=""
            loading="lazy"
          />
          <span className="eyebrow">
            {v.channel.number} · {v.channel.name}
          </span>
          <h3>
            {v.program?.entryType === "INTERSTITIAL"
              ? "Station Break"
              : v.program?.showTitle ||
                v.program?.title ||
                "Schedule not ready"}
          </h3>
          {v.program?.showTitle && (
            <p>
              {v.program.title} · S{v.program.season} E{v.program.episode}
            </p>
          )}
          <progress max="100" value={v.progress} />
          <p>Next · {v.next?.title ?? "Not scheduled"}</p>
          <div className="viewer-card-actions">
            <button
              className="primary"
              disabled={!v.program}
              onClick={() => tune(v.channel.id)}
            >
              Watch Live
            </button>
            {v.program && (
              <button
                onClick={() =>
                  setDetails(v.program!.followingProgramId || v.program!.id)
                }
              >
                Details
              </button>
            )}
            <FavoriteButton
              id={v.channel.id}
              favorite={favorites.includes(v.channel.id)}
              csrf={csrf}
              onChange={() => void refreshFavorites()}
            />
          </div>
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
      <button
        className="primary"
        onClick={() => {
          window.location.hash = "guide";
        }}
      >
        Open Live Guide
      </button>
      {items.some((v) => v.channel.id === last) && (
        <button className="primary" onClick={() => tune(last!)}>
          Watch Last Channel
        </button>
      )}
      {error && <p role="status">{error}</p>}
      <ForYou csrf={csrf} />
      <h2 className="on-now-heading">On Now</h2>
      {cards(items.filter((v) => !v.channel.recentKind))}
      <h2 className="on-now-heading">Favorite Channels</h2>
      {favorites.length ? (
        cards(items.filter((v) => favorites.includes(v.channel.id)))
      ) : (
        <p>Choose ☆ on a channel to make it yours.</p>
      )}
      <h2 className="on-now-heading">Starting Soon</h2>
      <div className="starting-list">
        {soon?.channels
          .flatMap((c) => c.programs.map((p) => ({ c, p })))
          .sort((a, b) => a.p.start - b.p.start)
          .map(
            ({ c, p }: { c: WindowData["channels"][number]; p: Program }) => (
              <article key={p.id}>
                <span>
                  {c.number} · {c.name}
                </span>
                <h3>{p.showTitle || p.title}</h3>
                <p>{timeLabel(p.start, soon.timezone)}</p>
                <button onClick={() => setDetails(p.id)}>View Details</button>
              </article>
            ),
          )}
      </div>
      {soon && !soon.channels.some((c) => c.programs.length) && (
        <p>
          No programs start in the next 30 minutes. Settle into what is on now.
        </p>
      )}
      <h2 className="on-now-heading">Channel Groups</h2>
      <div className="viewer-group-row">
        {soon?.groups.map((g) => (
          <button
            key={g.id}
            onClick={() => {
              chooseGuideGroup(userId, g.id);
              window.location.hash = "guide";
            }}
          >
            {g.name}
          </button>
        ))}
      </div>
      {items.some((v) => v.channel.recentKind) && (
        <>
          <h2 className="on-now-heading">Fresh in your collection</h2>
          {cards(items.filter((v) => !!v.channel.recentKind))}
        </>
      )}
      {details && (
        <ProgramDetails
          id={details}
          csrf={csrf}
          onClose={() => setDetails(null)}
          onTune={watch}
          onFavorite={() => void refreshFavorites()}
        />
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
