import { useEffect, useState } from "react";
import { guideAPI, type Detail, type TuneAction } from "./api";
import { Dialog, FavoriteButton } from "./Dialog";
import { timeLabel, stateAt, useServerClock } from "./timeline";
export function ProgramDetails({
  id,
  csrf,
  onClose,
  onTune,
  onFavorite,
}: {
  id: string;
  csrf: string;
  onClose: () => void;
  onTune: TuneAction;
  onFavorite?: () => void;
}) {
  const [v, setV] = useState<Detail | null>(null),
    [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    guideAPI
      .detail(id)
      .then((d) => {
        if (live) setV(d);
      })
      .catch(() => {
        if (live)
          setError(
            "Program details are unavailable. The retained schedule may have changed.",
          );
      });
    return () => {
      live = false;
    };
  }, [id]);
  const now = useServerClock(v?.serverTime ?? 0);
  const status = v ? stateAt(v, now) : "";
  return (
    <Dialog
      label="Program details"
      onClose={onClose}
      className="program-details"
    >
      {error ? (
        <p role="alert">{error}</p>
      ) : !v ? (
        <p role="status">Loading program…</p>
      ) : (
        <>
          <img className="detail-backdrop" src={v.background} alt="" />
          <div className="detail-content">
            <img className="detail-poster" src={v.artwork} alt="" />
            <div>
              <span className="eyebrow">
                {v.channel.number} · {v.channel.name} ·{" "}
                {status === "CURRENT"
                  ? "ON AIR"
                  : status === "PAST"
                    ? "PREVIOUSLY AIRED"
                    : "COMING UP"}
              </span>
              <h1>{v.showTitle || v.title}</h1>
              {v.showTitle && (
                <h2>
                  S{String(v.season ?? 0).padStart(2, "0")} E
                  {String(v.episode ?? 0).padStart(2, "0")} · {v.title}
                </h2>
              )}
              <p>
                {v.year} {v.contentRating}{" "}
                {v.rating != null && `· ★ ${v.rating.toFixed(1)}`} ·{" "}
                {Math.round(v.durationMs / 60000)} min
              </p>
              <p>
                {new Date(v.start).toLocaleDateString("en-US", {
                  timeZone: v.timezone,
                  month: "short",
                  day: "numeric",
                })}{" "}
                · {timeLabel(v.start, v.timezone)} –{" "}
                {timeLabel(v.end, v.timezone)} · {v.timezone}
              </p>
              <p>{v.summary || "No description available."}</p>
              <p>{v.genres.join(" · ")}</p>
              {v.channel.recentKind && v.addedAt && (
                <p>
                  Added{" "}
                  {new Date(v.addedAt * 1000).toLocaleDateString("en-US", {
                    timeZone: v.timezone,
                  })}
                </p>
              )}
              {status === "CURRENT" && (
                <progress
                  max={100}
                  value={Math.min(100, ((now - v.start) / v.durationMs) * 100)}
                />
              )}
              <div className="detail-actions">
                {status === "CURRENT" && (
                  <button
                    className="primary"
                    onClick={() => onTune(v.channelId)}
                  >
                    Watch Live
                  </button>
                )}
                {status !== "FUTURE" && v.available && (
                  <button onClick={() => onTune(v.channelId, v.id)}>
                    Watch From Beginning
                  </button>
                )}
                <FavoriteButton
                  id={v.channelId}
                  favorite={v.channel.favorite}
                  csrf={csrf}
                  onChange={() => onFavorite?.()}
                />
              </div>
              {!v.available && <p>This program is currently unavailable.</p>}
              {status === "FUTURE" && (
                <p>Available to watch when this program airs.</p>
              )}
            </div>
          </div>
        </>
      )}
    </Dialog>
  );
}
