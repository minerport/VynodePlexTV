import { useCallback, useEffect, useRef, useState } from "react";
import { guideAPI, type WindowData, type TuneAction } from "./api";
import { ProgramDetails } from "./ProgramDetails";
import { FavoriteButton } from "./Dialog";
import { GuideProgramCell, GuideTimeHeader } from "./GuideCells";
import {
  HOUR,
  ROW,
  position,
  dayLabel,
  dayStart,
  useServerClock,
} from "./timeline";
import "./guide.css";
type Memory = {
  start: number;
  group: string;
  favorites: boolean;
  offset: number;
  channel: string;
  at: number;
  scroll: number;
};
const memories = new Map<string, Memory>();
export function chooseGuideGroup(user: string, group: string) {
  const old = memories.get(user);
  memories.set(user, {
    start: old?.start ?? Date.now(),
    group,
    favorites: false,
    offset: 0,
    channel: "",
    at: Date.now(),
    scroll: 0,
  });
}
export function GuideGrid({
  csrf,
  userId,
  onTune,
  initialChannel,
}: {
  csrf: string;
  userId: string;
  onTune: TuneAction;
  initialChannel?: string;
}) {
  const [saved] = useState(() => memories.get(userId));
  const [start, setStart] = useState(
      () => saved?.start ?? Math.floor(Date.now() / 1800000) * 1800000,
    ),
    [group, setGroup] = useState(saved?.group ?? ""),
    [favorites, setFavorites] = useState(saved?.favorites ?? false),
    [offset, setOffset] = useState(saved?.offset ?? 0);
  const [data, setData] = useState<WindowData | null>(null),
    [error, setError] = useState(""),
    [details, setDetails] = useState<string | null>(null),
    [scroll, setScroll] = useState(saved?.scroll ?? 0);
  const [selection, setSelection] = useState(() => ({
    channel: initialChannel ?? saved?.channel ?? "",
    at: saved?.at ?? Date.now(),
  }));
  const daysRef = useRef<HTMLDivElement>(null);
  const nowButton = useRef<HTMLButtonElement>(null);
  const viewport = useRef<HTMLDivElement>(null),
    pendingFocus = useRef(true),
    initialized = useRef(false),
    requestEpoch = useRef(0);
  const end = start + 3 * HOUR;
  const now = useServerClock(data?.serverTime ?? 0);
  const load = useCallback(async () => {
    const epoch = ++requestEpoch.current;
    try {
      const v = await guideAPI.window(start, end, group, favorites, offset);
      if (epoch !== requestEpoch.current) return;
      setData(v);
      setError("");
      if (!initialized.current) {
        initialized.current = true;
        if (!saved) {
          setStart(Math.floor(v.serverTime / 1800000) * 1800000);
          setSelection((s) => ({ ...s, at: v.serverTime }));
        }
      }
    } catch (e) {
      if (epoch !== requestEpoch.current) return;
      setError(e instanceof Error ? e.message : "Guide unavailable");
    }
  }, [start, end, group, favorites, offset, saved]);
  useEffect(() => {
    void Promise.resolve().then(load);
    const t = setInterval(() => void load(), 30000);
    const invalidate = () => {
      requestEpoch.current++;
    };
    return () => {
      clearInterval(t);
      invalidate();
    };
  }, [load]);
  useEffect(() => {
    memories.set(userId, {
      start,
      group,
      favorites,
      offset,
      ...selection,
      scroll,
    });
  }, [userId, start, group, favorites, offset, selection, scroll]);
  const channels = data?.channels ?? [],
    zone = data?.timezone ?? "UTC";
  const selectedChannel =
    channels.find((c) => c.id === selection.channel) ?? channels[0];
  const selectedProgram =
    selectedChannel?.programs.find(
      (p) => p.start <= selection.at && p.end > selection.at,
    ) ??
    selectedChannel?.programs.find((p) => p.end > selection.at) ??
    selectedChannel?.programs.at(-1);
  useEffect(() => {
    if (!pendingFocus.current || details || !data) return;
    if (!selectedProgram) {
      nowButton.current?.focus();
      pendingFocus.current = false;
      return;
    }
    const el = viewport.current?.querySelector<HTMLButtonElement>(
      `[data-program="${selectedProgram.id}"]`,
    );
    if (el) {
      el.focus();
      el.scrollIntoView({ block: "nearest", inline: "nearest" });
      pendingFocus.current = false;
    }
  }, [data, selection, details, selectedProgram]);
  useEffect(() => {
    const center = () => {
      const parent = daysRef.current;
      const active = parent?.querySelector<HTMLElement>(
        '[aria-pressed="true"]',
      );
      if (parent && active)
        parent.scrollLeft =
          active.offsetLeft - parent.clientWidth / 2 + active.clientWidth / 2;
    };
    center();
    window.addEventListener("resize", center);
    return () => window.removeEventListener("resize", center);
  }, [start, data?.timezone]);
  function select(channel: string, at: number, focus = true) {
    pendingFocus.current = focus;
    setSelection({ channel, at });
    const i = channels.findIndex((c) => c.id === channel);
    if (viewport.current && i >= 0) {
      const top = i * ROW;
      if (
        top < viewport.current.scrollTop ||
        top + ROW >
          viewport.current.scrollTop + viewport.current.clientHeight - 60
      ) {
        viewport.current.scrollTop = Math.max(0, top - ROW);
        setScroll(viewport.current.scrollTop);
      }
    }
  }
  function move(key: string) {
    if (!selectedChannel || !selectedProgram) return;
    const row = channels.indexOf(selectedChannel),
      index = selectedChannel.programs.indexOf(selectedProgram);
    if (key === "ArrowUp" || key === "ArrowDown") {
      const delta = key === "ArrowDown" ? 1 : -1;
      const c = channels[row + delta];
      if (c) select(c.id, selection.at);
      else if (delta > 0 && data?.more) {
        setOffset(offset + 50);
        select("", selection.at);
      } else if (delta < 0 && offset > 0) {
        setOffset(offset - 50);
        select("", selection.at);
      }
    } else {
      const delta = key === "ArrowRight" ? 1 : -1,
        p = selectedChannel.programs[index + delta];
      if (p) {
        select(selectedChannel.id, p.start);
        if (p.start >= end) setStart(start + 3 * HOUR);
      } else {
        const at = delta > 0 ? selectedProgram.end : selectedProgram.start - 1;
        if (at >= now - 7 * 24 * HOUR && at <= now + 14 * 24 * HOUR) {
          setStart(Math.floor(at / 1800000) * 1800000);
          select(selectedChannel.id, at);
        }
      }
    }
  }
  function jump(at: number) {
    setStart(Math.floor(at / 1800000) * 1800000);
    select(selection.channel, at);
    if (viewport.current) viewport.current.scrollLeft = 0;
  }
  const first = Math.max(0, Math.floor(scroll / ROW) - 2),
    last = first + 12;
  const activeIndex = channels.indexOf(selectedChannel);
  return (
    <section className="guide-screen" aria-label="TV Guide">
      <div className="guide-heading">
        <div>
          <span className="eyebrow">YOUR CHANNELS. YOUR TIME.</span>
          <h1>Live guide</h1>
          <p>
            {dayLabel(start, zone)} · {zone}
          </p>
        </div>
        <button ref={nowButton} className="primary" onClick={() => jump(now)}>
          NOW
        </button>
      </div>
      <div className="guide-toolbar">
        <button
          aria-pressed={!favorites && !group}
          onClick={() => {
            setFavorites(false);
            setGroup("");
            setOffset(0);
          }}
        >
          All channels
        </button>
        <button
          aria-pressed={favorites}
          onClick={() => {
            setFavorites(!favorites);
            setGroup("");
            setOffset(0);
          }}
        >
          ★ Favorites
        </button>
        {data?.groups.map((g) => (
          <button
            key={g.id}
            aria-pressed={group === g.id}
            onClick={() => {
              setGroup(g.id);
              setFavorites(false);
              setOffset(0);
            }}
          >
            {g.name}
          </button>
        ))}
      </div>
      <div className="guide-days" ref={daysRef}>
        {Array.from({ length: 22 }, (_, i) => i - 7).map((d) => (
          <button
            key={d}
            aria-pressed={
              dayLabel(start, zone) ===
              dayLabel(dayStart(now || start, zone, d), zone)
            }
            onClick={() =>
              jump(
                dayStart(now || start, zone, d) +
                  (d === 0
                    ? Math.max(0, now - dayStart(now, zone))
                    : 18 * HOUR),
              )
            }
          >
            {d === 0
              ? "Today"
              : d === -1
                ? "Yesterday"
                : d === 1
                  ? "Tomorrow"
                  : dayLabel(dayStart(now || start, zone, d), zone)}
          </button>
        ))}
      </div>
      <div className="guide-toolbar">
        <button
          disabled={start <= now - 7 * 24 * HOUR}
          onClick={() => jump(start - 3 * HOUR)}
        >
          ← Earlier
        </button>
        <button
          disabled={start >= now + 14 * 24 * HOUR}
          onClick={() => jump(end)}
        >
          Later →
        </button>
        <span>← → Programs · ↑ ↓ Channels · Enter Details</span>
      </div>
      {error && (
        <p role="alert">
          {error} <button onClick={() => void load()}>Retry guide</button>
        </p>
      )}
      {!data && !error && <p role="status">Loading your television guide…</p>}
      {data && channels.length === 0 && (
        <div className="guide-empty">
          <h2>
            {favorites
              ? "Your favorites start here"
              : group
                ? "No channels in this group"
                : "No programming available"}
          </h2>
          <p>
            {favorites
              ? "Favorite a channel from its program details, then return here."
              : "Choose another group or ask an administrator to generate channel schedules."}
          </p>
        </div>
      )}
      {channels.length > 0 && (
        <div
          className="guide-viewport"
          tabIndex={-1}
          ref={viewport}
          onScroll={(e) => setScroll(e.currentTarget.scrollTop)}
          role="grid"
          aria-label="Program timeline"
          aria-rowcount={channels.length}
          data-spatial
        >
          <div
            className="guide-canvas"
            style={{
              width: `calc(var(--channel-width) + ${position(end, start)}px)`,
              height: 60 + channels.length * ROW,
            }}
          >
            <div className="guide-header">
              <div className="guide-corner">CHANNEL</div>
              <GuideTimeHeader start={start} end={end} zone={zone} now={now} />
            </div>
            {channels.map((c, i) =>
              (i >= first && i < last) || i === activeIndex ? (
                <div
                  role="row"
                  className="guide-row"
                  key={c.id}
                  style={{ top: 60 + i * ROW, height: ROW }}
                >
                  <div className="guide-channel">
                    <img src={c.logo} alt="" loading="lazy" />
                    <div>
                      <strong>{c.number}</strong>
                      <span>{c.name}</span>
                    </div>
                    <FavoriteButton
                      id={c.id}
                      favorite={c.favorite}
                      csrf={csrf}
                      onChange={() => {
                        pendingFocus.current = true;
                        void load();
                      }}
                    />
                  </div>
                  <div className="guide-track">
                    {c.programs.length ? (
                      c.programs.map((p) => (
                        <GuideProgramCell
                          key={p.id}
                          p={p}
                          start={start}
                          end={end}
                          now={now}
                          zone={zone}
                          selected={p.id === selectedProgram?.id}
                          onFocus={() =>
                            select(
                              c.id,
                              Math.max(
                                p.start,
                                Math.min(selection.at, p.end - 1),
                              ),
                              false,
                            )
                          }
                          onSelect={() => {
                            select(
                              c.id,
                              Math.max(
                                p.start,
                                Math.min(selection.at, p.end - 1),
                              ),
                              false,
                            );
                            setDetails(p.id);
                          }}
                          onKey={move}
                        />
                      ))
                    ) : (
                      <span className="guide-gap">
                        No retained programming in this window
                      </span>
                    )}
                  </div>
                </div>
              ) : null,
            )}
            {now >= start && now < end && (
              <div
                className="now-line"
                aria-label="Current time"
                style={{
                  left: `calc(var(--channel-width) + ${position(now, start)}px)`,
                  height: channels.length * ROW,
                }}
              />
            )}
          </div>
        </div>
      )}
      {data && (offset > 0 || data.more) && (
        <div className="guide-toolbar">
          <button
            disabled={offset === 0}
            onClick={() => {
              setOffset(Math.max(0, offset - 50));
              setScroll(0);
            }}
          >
            Previous channels
          </button>
          <button
            disabled={!data.more}
            onClick={() => {
              setOffset(offset + 50);
              setScroll(0);
            }}
          >
            More channels
          </button>
        </div>
      )}
      {data?.warnings.map((w) => (
        <p role="status" key={w}>
          {w}
        </p>
      ))}
      {details && (
        <ProgramDetails
          id={details}
          csrf={csrf}
          onClose={() => {
            pendingFocus.current = true;
            setDetails(null);
          }}
          onTune={onTune}
          onFavorite={() => void load()}
        />
      )}
    </section>
  );
}
