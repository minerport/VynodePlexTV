import { useEffect, useState } from "react";
import { request } from "../api";
import { type Channel, type Program } from "../guide/api";
import { ProgramDetails } from "../guide/ProgramDetails";
import { watch } from "../playback/api";
interface Recommendation {
  channel: Channel;
  program: Program;
  reasons: string[];
  components?: Record<string, number>;
  score?: number;
}
export function ForYou({
  csrf,
  expanded = false,
}: {
  csrf: string;
  expanded?: boolean;
}) {
  const [items, setItems] = useState<Recommendation[]>([]),
    [cold, setCold] = useState(true),
    [error, setError] = useState(""),
    [details, setDetails] = useState<string | null>(null),
    [debug, setDebug] = useState(false);
  useEffect(() => {
    let live = true;
    const load = () =>
      request<{ items: Recommendation[]; coldStart: boolean }>(
        `/personalization/${expanded ? "whats-good" : "for-you"}${debug ? "?debug=true" : ""}`,
      )
        .then((v) => {
          if (live) {
            setItems(v.items);
            setCold(v.coldStart);
            setError("");
          }
        })
        .catch(() => {
          if (live)
            setError("Your recommendations are temporarily unavailable.");
        });
    void load();
    const t = setInterval(() => void load(), 60000);
    return () => {
      live = false;
      clearInterval(t);
    };
  }, [expanded, debug]);
  return (
    <section
      aria-label={expanded ? "What's Good Right Now" : "For You Right Now"}
    >
      {expanded ? <h1>What’s Good Right Now</h1> : <h2>For You Right Now</h2>}
      <p>
        {cold
          ? "Start with your favorites and what’s on. After a few meaningful visits, your viewing helps shape this row."
          : "Your favorites and viewing, matched to what’s already on."}
      </p>
      {error && <p role="status">{error}</p>}
      {expanded && (
        <label className="personal-explain">
          <input
            type="checkbox"
            checked={debug}
            onChange={(e) => setDebug(e.target.checked)}
          />{" "}
          Show my ranking explanation details
        </label>
      )}
      {!items.length && !error && (
        <p>
          Nothing scheduled right now. Your channels will appear here when
          programming is ready.
        </p>
      )}
      <div className="on-now-grid">
        {items.slice(0, expanded ? 24 : 6).map((v) => (
          <article className="on-now-card" key={v.program.id}>
            <img
              className="program-art"
              src={v.program.artwork}
              alt=""
              loading="lazy"
            />
            <span className="eyebrow">
              {v.channel.number} · {v.channel.name}
            </span>
            <h3>{v.program.showTitle || v.program.title}</h3>
            {v.program.showTitle && <p>{v.program.title}</p>}
            <p>{v.reasons.slice(0, 2).join(" · ")}</p>
            <progress max="100" value={v.program.progress} />
            <div className="viewer-card-actions">
              {v.program.state === "CURRENT" && (
                <button className="primary" onClick={() => watch(v.channel.id)}>
                  Watch Live
                </button>
              )}
              <button onClick={() => setDetails(v.program.id)}>Details</button>
            </div>
            {expanded && debug && (
              <details>
                <summary>Ranking version 1 · {v.score?.toFixed(1)}</summary>
                <ul>
                  {Object.entries(v.components ?? {}).map(([key, value]) => (
                    <li key={key}>
                      {key.replaceAll("_", " ")}: {value.toFixed(1)}
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </article>
        ))}
      </div>
      {!expanded && <a href="#whats-good">Explore What’s Good Right Now →</a>}
      {details && (
        <ProgramDetails
          id={details}
          csrf={csrf}
          onClose={() => setDetails(null)}
          onTune={watch}
        />
      )}
    </section>
  );
}
