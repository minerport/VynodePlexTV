import type { Program } from "./api";
import { geometry, stateAt, timeLabel, position } from "./timeline";
export function GuideProgramCell({
  p,
  start,
  end,
  now,
  zone,
  selected,
  onFocus,
  onSelect,
  onKey,
}: {
  p: Program;
  start: number;
  end: number;
  now: number;
  zone: string;
  selected: boolean;
  onFocus: () => void;
  onSelect: () => void;
  onKey: (key: string) => void;
}) {
  const g = geometry(p.start, p.end, start, end),
    state = stateAt(p, now);
  return (
    <button
      role="gridcell"
      data-program={p.id}
      tabIndex={selected ? 0 : -1}
      className={`guide-program ${state.toLowerCase()}`}
      style={{ ...g, paddingInline: g.width < 40 ? 0 : 12 }}
      onFocus={onFocus}
      onClick={onSelect}
      onKeyDown={(e) => {
        if (
          ["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Enter"].includes(
            e.key,
          )
        ) {
          e.preventDefault();
          e.stopPropagation();
          if (e.key === "Enter") onSelect();
          else onKey(e.key);
        }
      }}
      aria-label={`${p.showTitle || p.title}${p.showTitle ? " · " + p.title : ""}, ${timeLabel(p.start, zone)}, ${state}`}
    >
      <strong>{p.showTitle || p.title}</strong>
      {g.width > 130 && p.showTitle && (
        <span>
          S{p.season} E{p.episode} · {p.title}
        </span>
      )}
      {g.width > 200 && (
        <small>
          {timeLabel(p.start, zone)} – {timeLabel(p.end, zone)}
        </small>
      )}
      {state === "CURRENT" && (
        <>
          <small className="on-air">ON AIR</small>
          <progress max={100} value={((now - p.start) / p.durationMs) * 100} />
        </>
      )}
    </button>
  );
}
export function GuideTimeHeader({
  start,
  end,
  zone,
  now,
}: {
  start: number;
  end: number;
  zone: string;
  now: number;
}) {
  const ticks = [];
  for (let t = Math.ceil(start / 1800000) * 1800000; t < end; t += 1800000)
    ticks.push(t);
  return (
    <div className="guide-time">
      {ticks.map((t) => (
        <span key={t} style={{ left: position(t, start) }}>
          {timeLabel(t, zone)}
        </span>
      ))}
      {now >= start && now < end && (
        <b className="now-label" style={{ left: position(now, start) }}>
          NOW
        </b>
      )}
    </div>
  );
}
