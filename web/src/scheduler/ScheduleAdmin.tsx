import { useCallback, useEffect, useRef, useState } from "react";
import { errorText } from "../channels/api";
import {
  scheduleAPI,
  localDay,
  shiftDay,
  displayTime,
  type ScheduleStatus,
  type ScheduleSettings,
  type ProgramPage,
  type ScheduleJob,
} from "./api";
import "./schedule.css";
export function ScheduleAdmin({
  csrf,
  onClose,
  initialId,
}: {
  csrf: string;
  onClose: () => void;
  initialId?: string;
}) {
  const [selected, setSelected] = useState(initialId ?? "");
  const [rows, setRows] = useState<ScheduleStatus[]>([]);
  const [error, setError] = useState("");
  const [offset, setOffset] = useState(0);
  const close = useRef<HTMLButtonElement>(null);
  const load = useCallback(async () => {
    try {
      setRows(await scheduleAPI.list(offset));
      setError("");
    } catch (e) {
      setError(errorText(e));
    }
  }, [offset]);
  useEffect(() => {
    const t = setTimeout(() => void load(), 0);
    const interval = setInterval(() => void load(), 10000);
    return () => {
      clearTimeout(t);
      clearInterval(interval);
    };
  }, [load]);
  useEffect(() => {
    close.current?.focus();
  }, [selected]);
  useEffect(() => {
    const back = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopImmediatePropagation();
        if (selected && !initialId) setSelected("");
        else onClose();
      }
    };
    window.addEventListener("keydown", back, true);
    return () => window.removeEventListener("keydown", back, true);
  }, [selected, initialId, onClose]);
  return (
    <section className="channel-page schedule-page">
      <button
        ref={close}
        onClick={() => (selected && !initialId ? setSelected("") : onClose())}
      >
        {selected && !initialId ? "All schedules" : "Back to channels"}
      </button>
      <span className="eyebrow">PROGRAMMING STUDIO</span>
      <h1>
        {selected
          ? "A stable broadcast timeline."
          : "Your channels, around the clock."}
      </h1>
      <p>
        Continuous programming from your local library. Every start time is
        saved.
      </p>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {selected ? (
        <ScheduleDetail key={selected} id={selected} csrf={csrf} />
      ) : (
        <>
          <div className="schedule-grid">
            {rows.map((s) => (
              <article className="channel-panel" key={s.channelId}>
                <span className="eyebrow">
                  {s.number} · {s.mode.replaceAll("_", " ")}
                </span>
                <h2>{s.name}</h2>
                <span className="schedule-badge">
                  {s.status.replaceAll("_", " ")}
                </span>
                <p>
                  {s.programCount.toLocaleString()} programs ·{" "}
                  {s.schedulable.toLocaleString()} eligible items
                </p>
                <p>
                  {displayTime(s.coverageStart, s.timezone)}
                  <br />→ {displayTime(s.coverageEnd, s.timezone)}
                </p>
                <small>
                  Last generation: {displayTime(s.lastGenerated, s.timezone)}
                </small>
                <div className="channel-actions">
                  <button onClick={() => setSelected(s.channelId)}>
                    Open schedule
                  </button>
                  <button
                    onClick={() =>
                      void scheduleAPI
                        .run(s.channelId, "generate", csrf)
                        .then(load)
                        .catch((e) => setError(errorText(e)))
                    }
                  >
                    Generate
                  </button>
                  <button
                    onClick={() =>
                      void scheduleAPI
                        .run(s.channelId, "repair", csrf)
                        .then(load)
                        .catch((e) => setError(errorText(e)))
                    }
                  >
                    Repair
                  </button>
                </div>
              </article>
            ))}
          </div>
          {rows.length === 0 && (
            <p>
              No channels configured. Create a channel to start programming.
            </p>
          )}
          <div className="channel-actions">
            <button
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 30))}
            >
              Previous channels
            </button>
            <button
              disabled={rows.length < 30}
              onClick={() => setOffset(offset + 30)}
            >
              More channels
            </button>
          </div>
        </>
      )}
    </section>
  );
}
function ScheduleDetail({ id, csrf }: { id: string; csrf: string }) {
  const [settings, setSettings] = useState<ScheduleSettings | null>(null);
  const [status, setStatus] = useState<ScheduleStatus | null>(null);
  const [day, setDay] = useState("");
  const [offset, setOffset] = useState(0);
  const [page, setPage] = useState<ProgramPage | null>(null);
  const [jobs, setJobs] = useState<ScheduleJob[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState<"seed" | "regenerate" | null>(null);
  useEffect(() => {
    let alive = true;
    void scheduleAPI
      .settings(id)
      .then((v) => {
        if (alive) {
          setSettings(v);
          setDay(localDay(v.timezone));
        }
      })
      .catch((e) => {
        if (alive) setError(errorText(e));
      });
    return () => {
      alive = false;
    };
  }, [id]);
  const refresh = useCallback(async () => {
    if (!day) return;
    const [s, p, j] = await Promise.all([
      scheduleAPI.status(id),
      scheduleAPI.programs(id, day, offset),
      scheduleAPI.jobs(id),
    ]);
    setStatus(s);
    setPage(p);
    setJobs(j);
  }, [id, day, offset]);
  useEffect(() => {
    const t = setTimeout(
      () => void refresh().catch((e) => setError(errorText(e))),
      0,
    );
    const i = setInterval(
      () => void refresh().catch((e) => setError(errorText(e))),
      4000,
    );
    return () => {
      clearTimeout(t);
      clearInterval(i);
    };
  }, [refresh]);
  async function run(action: string) {
    setBusy(true);
    setError("");
    try {
      await scheduleAPI.run(id, action, csrf);
      setNotice(
        "Schedule job queued. Current and past programs are protected.",
      );
      setConfirm(null);
      await refresh();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function save() {
    if (!settings) return;
    setBusy(true);
    setError("");
    try {
      setSettings(await scheduleAPI.save(id, settings, csrf));
      setNotice(
        "Settings saved. Repair applies relevant changes to future programs.",
      );
      await refresh();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  function go(day: string) {
    setDay(day);
    setOffset(0);
  }
  if (!settings)
    return <p role="status">{error || "Loading schedule settings…"}</p>;
  return (
    <>
      <h2>
        {status?.number} · {status?.name}
      </h2>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="success" role="status">
          {notice}
        </p>
      )}
      <div className="schedule-summary channel-panel">
        <strong>{status?.status.replaceAll("_", " ")}</strong>
        <span>
          {status?.resolved.toLocaleString()} resolved →{" "}
          {status?.schedulable.toLocaleString()} schedulable
        </span>
        <span>
          {status?.programCount.toLocaleString()} programs · revision{" "}
          {status?.revision}
        </span>
        <span>
          Integrity:{" "}
          {status?.integrity.length ? status.integrity.join(" · ") : "OK"}
        </span>
        <p>
          {displayTime(status?.coverageStart ?? null, settings.timezone)} →{" "}
          {displayTime(status?.coverageEnd ?? null, settings.timezone)}
        </p>
        {status?.warnings.map((w) => (
          <p className="notice" key={w}>
            {w}
          </p>
        ))}
        {status?.lastError && <p className="error">{status.lastError}</p>}
      </div>
      <details className="channel-panel">
        <summary>Scheduling settings</summary>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <div className="schedule-fields">
            <label>
              Scheduling mode
              <select
                value={settings.mode}
                onChange={(e) =>
                  setSettings({ ...settings, mode: e.target.value })
                }
              >
                <option value="SEQUENTIAL">Sequential</option>
                <option value="RANDOM">Random</option>
                <option value="SMART_RANDOM">Smart Random</option>
              </select>
            </label>
            <label>
              Timezone
              <input
                required
                value={settings.timezone}
                onChange={(e) =>
                  setSettings({ ...settings, timezone: e.target.value })
                }
              />
            </label>
            <label>
              Past horizon (days)
              <input
                type="number"
                min="0"
                max="30"
                value={settings.pastDays}
                onChange={(e) =>
                  setSettings({ ...settings, pastDays: +e.target.value })
                }
              />
            </label>
            <label>
              Future horizon (days)
              <input
                type="number"
                min="1"
                max="30"
                value={settings.futureDays}
                onChange={(e) =>
                  setSettings({ ...settings, futureDays: +e.target.value })
                }
              />
            </label>
            {settings.mode === "SMART_RANDOM" && (
              <>
                <label>
                  Movie repeat window (days)
                  <input
                    type="number"
                    min="0"
                    max="365"
                    value={settings.movieRepeatDays}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        movieRepeatDays: +e.target.value,
                      })
                    }
                  />
                </label>
                <label>
                  Episode repeat window (days)
                  <input
                    type="number"
                    min="0"
                    max="365"
                    value={settings.episodeRepeatDays}
                    onChange={(e) =>
                      setSettings({
                        ...settings,
                        episodeRepeatDays: +e.target.value,
                      })
                    }
                  />
                </label>
              </>
            )}
          </div>
          <p>
            Sequential follows channel membership order. Random repeats a seeded
            full cycle. Smart Random prefers eligible items aired least recently
            and relaxes repeat windows when necessary.
          </p>
          <label>
            <input
              type="checkbox"
              checked={settings.enabled}
              onChange={(e) =>
                setSettings({ ...settings, enabled: e.target.checked })
              }
            />{" "}
            Maintain schedule automatically
          </label>
          <details>
            <summary>Advanced</summary>
            <label>
              Initial schedule anchor (UTC)
              <input
                type="datetime-local"
                step="1"
                required
                value={new Date(settings.anchorMs).toISOString().slice(0, 19)}
                onChange={(e) => {
                  const n = Date.parse(e.target.value + "Z");
                  if (Number.isFinite(n))
                    setSettings({ ...settings, anchorMs: n });
                }}
              />
            </label>
            <p>
              The anchor defines the initial timeline. Existing programs that
              have started remain protected.
            </p>
            <label>
              Random seed
              <input
                required
                maxLength={128}
                value={settings.seed}
                onChange={(e) =>
                  setSettings({ ...settings, seed: e.target.value })
                }
              />
            </label>
            <button type="button" onClick={() => setConfirm("seed")}>
              Regenerate seed
            </button>
            <label>
              <input
                type="checkbox"
                checked={settings.includeUnavailable}
                onChange={(e) =>
                  setSettings({
                    ...settings,
                    includeUnavailable: e.target.checked,
                  })
                }
              />{" "}
              Include unavailable members for planning
            </label>
          </details>
          <button className="primary" disabled={busy}>
            Save schedule settings
          </button>
        </form>
      </details>
      <div className="channel-actions">
        <button disabled={busy} onClick={() => void run("generate")}>
          Generate / extend
        </button>
        <button disabled={busy} onClick={() => void run("repair")}>
          Repair future
        </button>
        <button disabled={busy} onClick={() => setConfirm("regenerate")}>
          Regenerate future schedule
        </button>
      </div>
      {confirm && (
        <div
          className="channel-panel"
          role="alertdialog"
          aria-label="Confirm schedule change"
        >
          <h3>
            {confirm === "seed"
              ? "Change random seed?"
              : "Rebuild future programs?"}
          </h3>
          <p>
            {confirm === "seed"
              ? "A new seed changes future ordering when saved and repaired."
              : "All programs after the currently airing program will be rebuilt through the configured horizon. Past and current entries stay unchanged."}
          </p>
          <button
            autoFocus
            onClick={() => {
              if (confirm === "seed") {
                setSettings({ ...settings, seed: crypto.randomUUID() });
                setConfirm(null);
              } else void run("regenerate");
            }}
          >
            Confirm {confirm === "seed" ? "new seed" : "regeneration"}
          </button>
          <button onClick={() => setConfirm(null)}>Cancel</button>
        </div>
      )}
      <div className="channel-actions">
        <button onClick={() => go(shiftDay(day, -1))}>Previous day</button>
        <label>
          Schedule day
          <input
            type="date"
            value={day}
            onChange={(e) => {
              if (e.target.value) go(e.target.value);
            }}
          />
        </label>
        <button onClick={() => go(shiftDay(day, 1))}>Next day</button>
        <button onClick={() => go(localDay(status?.timezone ?? "UTC"))}>
          Today
        </button>
        <span>{settings.timezone}</span>
      </div>
      <ol className="schedule-programs">
        {page?.items.map((p) => (
          <li key={p.id} className="channel-panel">
            <time dateTime={new Date(p.start).toISOString()}>
              {displayTime(p.start, settings.timezone)}
              <small>→ {displayTime(p.end, settings.timezone)}</small>
            </time>
            <img loading="lazy" src={p.artwork} alt="" />
            <div>
              <h3>{p.showTitle || p.title}</h3>
              {p.showTitle && (
                <p>
                  S{String(p.season ?? 0).padStart(2, "0")} E
                  {String(p.episode ?? 0).padStart(2, "0")} · {p.title}
                </p>
              )}
              <small>
                {(p.durationMs / 60000).toFixed(1)} min · {p.policyReason}
              </small>
              {!p.available && (
                <p className="notice">
                  Media unavailable in the synchronized inventory
                </p>
              )}
            </div>
          </li>
        ))}
      </ol>
      {page?.items.length === 0 && (
        <p className="empty-panel">
          No programs for this day. Generate a schedule or choose a day inside
          its coverage.
        </p>
      )}
      <div className="channel-actions">
        <button
          disabled={offset === 0}
          onClick={() => setOffset(Math.max(0, offset - 100))}
        >
          Previous programs
        </button>
        <button disabled={!page?.more} onClick={() => setOffset(offset + 100)}>
          More programs
        </button>
      </div>
      <details className="channel-panel">
        <summary>Scheduler jobs</summary>
        {jobs.map((j) => (
          <p key={j.id}>
            <strong>
              {j.type.replaceAll("_", " ")} · {j.status}
            </strong>{" "}
            —{" "}
            {j.status === "RUNNING" || j.status === "QUEUED"
              ? "Preparing timeline…"
              : `${j.processed} generated, ${j.repaired} replaced`}
            {j.error && ` · ${j.error}`}
          </p>
        ))}
      </details>
    </>
  );
}
