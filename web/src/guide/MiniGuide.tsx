import { useEffect, useRef, useState } from "react";
import { guideAPI, type WindowData } from "./api";
import { Dialog } from "./Dialog";
export function MiniGuide({
  channelId,
  onClose,
  onTune,
}: {
  channelId: string;
  onClose: () => void;
  onTune: (id: string) => void;
}) {
  const [data, setData] = useState<WindowData | null>(null),
    [error, setError] = useState("");
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let live = true;
    const load = () =>
      guideAPI
        .now()
        .then((v) => {
          if (live) setData(v);
        })
        .catch(() => {
          if (live) setError("Mini-guide is temporarily unavailable");
        });
    void load();
    const t = setInterval(() => void load(), 30000);
    return () => {
      live = false;
      clearInterval(t);
    };
  }, []);
  useEffect(() => {
    ref.current
      ?.querySelector<HTMLButtonElement>(`[data-channel="${channelId}"]`)
      ?.focus();
  }, [channelId, data?.channels.length]);
  return (
    <Dialog label="Mini-guide" onClose={onClose} className="mini-guide">
      <h2>On now & next</h2>
      {error && <p role="alert">{error}</p>}
      <div ref={ref}>
        {data?.channels.map((c) => (
          <button
            key={c.id}
            data-channel={c.id}
            className={c.id === channelId ? "mini-selected" : ""}
            onFocus={(e) =>
              e.currentTarget.scrollIntoView({ block: "nearest" })
            }
            onClick={() => onTune(c.id)}
          >
            <img src={c.logo} alt="" />
            <span>
              <strong>
                {c.number} · {c.name}
              </strong>
              <span>
                {c.programs[0]?.showTitle ||
                  c.programs[0]?.title ||
                  "No program scheduled"}
              </span>
              <progress max={100} value={c.programs[0]?.progress ?? 0} />
              <small>
                Next · {c.programs[1]?.title ?? "Schedule unavailable"}
              </small>
            </span>
          </button>
        ))}
      </div>
      {data && !data.channels.length && <p>No channels available.</p>}
    </Dialog>
  );
}
