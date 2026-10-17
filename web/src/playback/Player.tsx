import { MiniGuide } from "../guide/MiniGuide";
import { GuideGrid } from "../guide/GuideGrid";
import { ProgramDetails } from "../guide/ProgramDetails";
import { Dialog, FavoriteButton } from "../guide/Dialog";
import { guideAPI } from "../guide/api";
import "../guide/guide.css";
import { useEffect, useRef, useState } from "react";
import { playbackAPI, watch, type OnNow } from "./api";
import { PlayerController, type PlayerState } from "./controller";
import "./player.css";
export function Player({
  channelId,
  programId,
  csrf,
  userId,
  onExit,
}: {
  channelId: string;
  programId?: string;
  csrf: string;
  userId?: string;
  onExit: () => void;
}) {
  const [panel, setPanel] = useState<"mini" | "guide" | "details" | null>(null);
  const [favorite, setFavorite] = useState(false);
  useEffect(() => {
    guideAPI
      .favorites()
      .then((v) => setFavorite(v.channelIds.includes(channelId)))
      .catch(() => {});
  }, [channelId]);
  function tuneOverlay(id: string, p?: string) {
    setPanel(null);
    if (id === channelId)
      void controller.current?.tune(id, p ? "FROM_BEGINNING" : "LIVE", p);
    else watch(id, p);
  }
  const video = useRef<HTMLVideoElement>(null),
    controller = useRef<PlayerController | null>(null);
  const [state, setState] = useState<PlayerState>({
    tune: null,
    loading: true,
    error: "",
    needsPlay: false,
    message: "",
  });
  const [channels, setChannels] = useState<OnNow[]>([]);
  const [visible, setVisible] = useState(true);
  const [digits, setDigits] = useState("");
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const numberTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const controls = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let live = true;
    void playbackAPI
      .onNow()
      .then((v) => {
        if (live) setChannels(v.items);
      })
      .catch(() =>
        setState((s) => ({
          ...s,
          message:
            "Channel list could not be loaded. Exit and retry to restore channel switching.",
        })),
      );
    return () => {
      live = false;
    };
  }, []);
  useEffect(() => {
    const c = new PlayerController(video.current!, csrf, setState);
    controller.current = c;
    return () => {
      c.dispose();
      controller.current = null;
    };
  }, [csrf]);
  useEffect(() => {
    void controller.current?.tune(
      channelId,
      programId ? "FROM_BEGINNING" : "LIVE",
      programId,
    );
  }, [channelId, programId]);
  useEffect(() => {
    if (userId && state.tune)
      localStorage.setItem(
        `afterglow-last-channel-${userId}`,
        state.tune.channel.id,
      );
  }, [userId, state.tune]);
  function reveal() {
    setVisible(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => {
      if (controls.current?.contains(document.activeElement))
        video.current?.focus();
      setVisible(false);
    }, 6500);
  }
  const switchChannel = (delta: number) => {
    if (!channels.length) return;
    const index = channels.findIndex((v) => v.channel.id === channelId);
    watch(
      channels[(index + delta + channels.length) % channels.length].channel.id,
    );
    reveal();
  };
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (panel || e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey)
        return;
      if (e.key.toLowerCase() === "g") {
        e.preventDefault();
        setPanel("mini");
        return;
      }
      if (e.key.toLowerCase() === "i") {
        e.preventDefault();
        setPanel("details");
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        onExit();
      } else if (
        ["ArrowUp", "PageUp", "ArrowDown", "PageDown"].includes(e.key)
      ) {
        e.preventDefault();
        switchChannel(e.key === "ArrowUp" || e.key === "PageUp" ? 1 : -1);
      } else if (e.key === " ") {
        e.preventDefault();
        controller.current?.toggle();
        reveal();
      } else if (
        e.key === "Enter" &&
        !(e.target instanceof HTMLButtonElement)
      ) {
        e.preventDefault();
        controller.current?.toggle();
        reveal();
      } else if (e.key === "ArrowLeft" || e.key === "ArrowRight") {
        if (e.target instanceof HTMLInputElement) return;
        e.preventDefault();
        reveal();
        const buttons = Array.from(
          controls.current?.querySelectorAll("button:not([disabled])") ?? [],
        ) as HTMLButtonElement[];
        const i = buttons.indexOf(document.activeElement as HTMLButtonElement);
        buttons[
          (i + (e.key === "ArrowRight" ? 1 : -1) + buttons.length) %
            buttons.length
        ]?.focus();
      } else if (/^\d$/.test(e.key)) {
        const next = (digits + e.key).slice(-5);
        setDigits(next);
        if (numberTimer.current) clearTimeout(numberTimer.current);
        numberTimer.current = setTimeout(() => {
          const c = channels.find((c) => c.channel.number === next);
          if (c) watch(c.channel.id);
          setDigits("");
        }, 1200);
      }
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  });
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
      if (numberTimer.current) clearTimeout(numberTimer.current);
    },
    [],
  );
  const t = state.tune,
    selected = channels.find((c) => c.channel.id === channelId),
    channel = t?.channel ?? selected?.channel,
    program = t?.program ?? selected?.program;
  const show = visible || state.loading || !!state.error || state.needsPlay;
  const time = (n: number) =>
    new Date(n).toLocaleTimeString([], {
      hour: "numeric",
      minute: "2-digit",
      timeZone: channel?.timezone,
    });
  return (
    <main
      className="tv-player"
      onMouseMove={reveal}
      onPointerDown={reveal}
      aria-label="Television player"
    >
      <video
        tabIndex={-1}
        ref={video}
        playsInline
        onPlaying={reveal}
        aria-label="Channel video"
      />
      {digits && <div className="channel-digits">{digits}</div>}
      {(state.loading || state.error || state.needsPlay || state.message) && (
        <div className="player-status" role={state.error ? "alert" : "status"}>
          <h2>
            {state.error
              ? "Let’s get you back on air"
              : state.needsPlay
                ? "Ready when you are"
                : state.loading
                  ? "Tuning in…"
                  : "Channel update"}
          </h2>
          <p>{state.error || state.message || channel?.name}</p>
          {state.error && (
            <button autoFocus onClick={() => void controller.current?.retry()}>
              Retry Playback
            </button>
          )}
          {state.needsPlay && (
            <button autoFocus onClick={() => void controller.current?.play()}>
              Play
            </button>
          )}
        </div>
      )}
      <div
        className={`player-chrome ${show ? "shown" : ""}`}
        inert={!show || !!panel}
      >
        <button className="player-exit" onClick={onExit}>
          ← Exit player
        </button>
        <section className="channel-overlay">
          <div className="channel-ident">
            {channel?.logo && (
              <img className="overlay-logo" src={channel.logo} alt="" />
            )}
            <span>{channel?.number ?? "—"}</span>
            <div>
              <p>{channel?.name ?? "Your channel"}</p>
              <small>
                {t?.origin === "CATCH_UP"
                  ? "CATCH-UP"
                  : t?.mode === "LIVE"
                    ? "● LIVE"
                    : t?.mode === "FROM_BEGINNING"
                      ? "FROM BEGINNING"
                      : t
                        ? `−${Math.max(1, Math.round((t.liveOffsetMs - t.positionMs) / 60000))}m · BEHIND LIVE`
                        : "CONNECTING"}
              </small>
            </div>
          </div>
          <h1>
            {program?.showTitle || program?.title || "Finding your frequency"}
          </h1>
          {program?.showTitle && (
            <p>
              S{String(program.season ?? 0).padStart(2, "0")} E
              {String(program.episode ?? 0).padStart(2, "0")} · {program.title}
            </p>
          )}
          {program && (
            <p>
              {time(program.start)} – {time(program.end)}
              {channel?.recentKind && t?.addedAt
                ? ` · Added ${Math.max(0, Math.floor((t.serverTime / 1000 - t.addedAt) / 86400))} days ago`
                : ""}
            </p>
          )}
          {program && (
            <progress
              max="100"
              value={
                t
                  ? Math.max(
                      0,
                      Math.min(
                        100,
                        ((t.serverTime - program.start) / program.durationMs) *
                          100,
                      ),
                    )
                  : (selected?.progress ?? 0)
              }
            />
          )}
          <p className="next-program">
            {t?.origin === "CATCH_UP" ? "ON LIVE CHANNEL NEXT" : "NEXT"} ·{" "}
            {t?.next?.title ?? selected?.next?.title ?? "Schedule unavailable"}
          </p>
          <div ref={controls} className="player-controls">
            <button onClick={() => switchChannel(-1)}>Channel −</button>
            <button onClick={() => controller.current?.toggle()}>
              Play / Pause
            </button>
            <button
              disabled={!t}
              onClick={() =>
                void controller.current?.tune(
                  channelId,
                  "FROM_BEGINNING",
                  t?.program.id,
                )
              }
            >
              Watch From Beginning
            </button>
            <button
              disabled={!t}
              onClick={() => void controller.current?.goLive()}
            >
              Go Live
            </button>
            <button onClick={() => switchChannel(1)}>Channel +</button>
            <button onClick={() => setPanel("mini")}>Mini-guide</button>
            <button onClick={() => setPanel("guide")}>Full Guide</button>
            <button disabled={!t} onClick={() => setPanel("details")}>
              Program Details
            </button>
            <FavoriteButton
              id={channelId}
              favorite={favorite}
              csrf={csrf}
              onChange={setFavorite}
            />
            <button
              onClick={() =>
                void document.documentElement
                  .requestFullscreen()
                  .catch(() => {})
              }
            >
              Full screen
            </button>
          </div>
          {t && (
            <label className="player-seek">
              Position
              <input
                aria-label="Playback position"
                type="range"
                min="0"
                max={Math.max(1, t.durationMs / 1000)}
                value={t.positionMs / 1000}
                onChange={(e) => {
                  if (video.current)
                    video.current.currentTime = +e.target.value;
                }}
              />
            </label>
          )}
          <small>
            ↑ ↓ Channel · Space Play / Pause · ← → Controls · Enter Select · Esc
            Exit · G Mini-guide · I Details · Type a channel number
          </small>
        </section>
      </div>
      {panel === "mini" && (
        <MiniGuide
          channelId={channelId}
          onClose={() => {
            setPanel(null);
            reveal();
          }}
          onTune={(id) => {
            tuneOverlay(id);
          }}
        />
      )}
      {panel === "guide" && (
        <Dialog
          label="Full Guide"
          className="player-guide"
          onClose={() => {
            setPanel(null);
            reveal();
          }}
        >
          <GuideGrid
            csrf={csrf}
            userId={userId ?? "viewer"}
            initialChannel={channelId}
            onTune={(id, p) => {
              tuneOverlay(id, p);
            }}
          />
        </Dialog>
      )}
      {panel === "details" && t && (
        <ProgramDetails
          id={t.program.id}
          csrf={csrf}
          onClose={() => {
            setPanel(null);
            reveal();
          }}
          onTune={(id, p) => {
            setPanel(null);
            void controller.current?.tune(id, p ? "FROM_BEGINNING" : "LIVE", p);
          }}
          onFavorite={() =>
            guideAPI
              .favorites()
              .then((v) => setFavorite(v.channelIds.includes(channelId)))
          }
        />
      )}
    </main>
  );
}
