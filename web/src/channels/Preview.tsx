import { useEffect, useState } from "react";
import { APIError } from "../api";
import {
  channelAPI,
  duration,
  errorText,
  type Preview as Result,
  type Source,
} from "./api";
import { Pager } from "./MediaPicker";
export function Preview({ source, csrf }: { source: Source; csrf: string }) {
  const [result, setResult] = useState<Result | null>(null);
  const [offset, setOffset] = useState(0);
  const [include, setInclude] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(true);
  useEffect(() => {
    let live = true;
    const t = setTimeout(() => {
      setBusy(true);
      channelAPI
        .preview(source, csrf, offset, include)
        .then((r) => {
          if (live) {
            setResult(r);
            setError("");
          }
        })
        .catch((e) => {
          if (live) {
            setResult(null);
            setError(
              e instanceof APIError && e.issues.length
                ? e.issues.map((i) => `${i.path}: ${i.message}`).join(" · ")
                : errorText(e),
            );
          }
        })
        .finally(() => {
          if (live) setBusy(false);
        });
    }, 300);
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [source, csrf, offset, include]);
  return (
    <section className="channel-preview">
      <span className="eyebrow">LOCAL LIBRARY PREVIEW</span>
      <h2>
        {busy
          ? "Checking membership…"
          : result
            ? `${result.count.toLocaleString()} playable items`
            : "Complete your source"}
      </h2>
      <p>
        Membership only. Programming times and playback arrive in a later phase.
      </p>
      <label className="check-label">
        <input
          type="checkbox"
          checked={include}
          onChange={(e) => {
            setInclude(e.target.checked);
            setOffset(0);
          }}
        />
        Include unavailable / inactive inventory
      </label>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {!busy && result?.count === 0 && (
        <p className="notice">
          No eligible media matches. You can save this channel and adjust its
          source later.
        </p>
      )}
      {result?.warnings.map((w) => (
        <p className="notice" key={w}>
          {w}
        </p>
      ))}
      <div className="preview-list">
        {result?.items.map((m) => (
          <article key={m.id}>
            <img
              loading="lazy"
              src={`/api/v1/media/${m.id}/artwork/poster`}
              alt=""
            />
            <div>
              <strong>
                {m.showTitle ? `${m.showTitle} · ` : ""}
                {m.title}
              </strong>
              <p>
                {m.type === "episode"
                  ? `S${m.season ?? "?"} E${m.episode ?? "?"}`
                  : m.year}{" "}
                · {duration(m.durationMs)}
              </p>
              <small>
                {m.library}
                {!m.eligible ? " · Unavailable / inactive" : ""}
              </small>
            </div>
          </article>
        ))}
      </div>
      {result && (
        <Pager offset={offset} count={result.count} onChange={setOffset} />
      )}
    </section>
  );
}
