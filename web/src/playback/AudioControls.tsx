import { useEffect, useRef, useState } from "react";

export function AudioControls({
  getVideo,
}: {
  getVideo: () => HTMLVideoElement | null;
}) {
  const [level, setLevel] = useState(100);
  const [error, setError] = useState("");
  const graph = useRef<{ context: AudioContext; gain: GainNode } | null>(null);
  useEffect(
    () => () => {
      void graph.current?.context.close();
    },
    [],
  );
  async function apply(next: number) {
    const element = getVideo();
    if (!element) return;
    try {
      if (next > 100 && !graph.current) {
        if (!window.AudioContext) throw new Error("unsupported");
        const context = new AudioContext();
        const source = context.createMediaElementSource(element);
        const gain = context.createGain();
        source.connect(gain);
        gain.connect(context.destination);
        graph.current = { context, gain };
      }
      if (graph.current) {
        await graph.current.context.resume();
        graph.current.gain.gain.value = Math.max(1, next / 100);
      }
      element.volume = Math.min(1, next / 100);
      element.muted = next === 0;
      setLevel(next);
      setError("");
    } catch {
      setError(
        "Audio boost is unavailable in this browser. Standard volume still works.",
      );
    }
  }
  return (
    <div
      className="player-audio"
      onKeyDown={(e) => {
        if (e.target instanceof HTMLInputElement && e.key.startsWith("Arrow"))
          e.stopPropagation();
      }}
    >
      <label>
        Volume {level}%
        <input
          aria-label="Volume"
          type="range"
          min="0"
          max="200"
          step="10"
          value={level}
          onChange={(e) => void apply(Number(e.target.value))}
        />
      </label>
      <button onClick={() => void apply(level === 0 ? 100 : 0)}>
        {level === 0 ? "Unmute" : "Mute"}
      </button>
      <button onClick={() => void apply(level > 100 ? 100 : 150)}>
        {level > 100 ? "Reset boost" : "Boost audio"}
      </button>
      {error && <span role="status">{error}</span>}
    </div>
  );
}
