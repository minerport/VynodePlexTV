import Hls from "hls.js";
import { playbackAPI, type Tune, type Capabilities } from "./api";
export interface PlayerState {
  tune: Tune | null;
  loading: boolean;
  error: string;
  needsPlay: boolean;
  message: string;
}
export function capabilities(video: HTMLVideoElement): Capabilities {
  const h264 = !!video.canPlayType('video/mp4; codecs="avc1.42E01E"');
  const aac = !!video.canPlayType('audio/mp4; codecs="mp4a.40.2"');
  return {
    clientType: "browser",
    containers: h264 ? ["mp4"] : [],
    videoCodecs: h264 ? ["h264"] : [],
    audioCodecs: aac ? ["aac"] : [],
    maxHeight: 1080,
    hls:
      Hls.isSupported() || !!video.canPlayType("application/vnd.apple.mpegurl"),
  };
}
export class PlayerController {
  state: PlayerState = {
    tune: null,
    loading: true,
    error: "",
    needsPlay: false,
    message: "",
  };
  private metadataAt = 0;
  private epoch = 0;
  private disposed = false;
  private hls: Hls | null = null;
  private timer: ReturnType<typeof setInterval>;
  private polling = false;
  private ready = false;
  private stalled: ReturnType<typeof setTimeout> | null = null;
  private ended = false;
  private target = "";
  private targetProgram: string | undefined;
  private force = false;
  private client = Array.from(crypto.getRandomValues(new Uint8Array(16)), (v) =>
    v.toString(16).padStart(2, "0"),
  ).join("");
  private listeners: [string, EventListener][] = [];
  constructor(
    private video: HTMLVideoElement,
    private csrf: string,
    private change: (state: PlayerState) => void,
  ) {
    this.timer = setInterval(() => void this.heartbeat(), 1000);
    this.listen("playing", () => {
      if (this.stalled) clearTimeout(this.stalled);
      this.ready = true;
      this.publish({ loading: false, needsPlay: false, error: "" });
      void this.heartbeat();
    });
    this.listen("pause", () => {
      if (this.ready && !this.video.ended) void this.heartbeat();
    });
    this.listen("seeked", () => {
      if (this.ready) void this.heartbeat();
    });
    this.listen("ended", () => void this.heartbeat(true));
    this.listen("error", () => void this.failed());
    this.listen("waiting", () => {
      this.publish({ loading: true });
      if (this.stalled) clearTimeout(this.stalled);
      this.stalled = setTimeout(() => void this.failed(), 15000);
    });
  }
  private listen(name: string, fn: EventListener) {
    this.video.addEventListener(name, fn);
    this.listeners.push([name, fn]);
  }
  private publish(p: Partial<PlayerState>) {
    if (this.disposed) return;
    this.state = { ...this.state, ...p };
    this.change(this.state);
  }
  private detach() {
    if (this.stalled) clearTimeout(this.stalled);
    this.ready = false;
    this.ended = false;
    this.hls?.destroy();
    this.hls = null;
    this.video.pause();
    this.video.removeAttribute("src");
    this.video.load();
  }
  async tune(channel: string, mode = "LIVE", programId?: string) {
    this.targetProgram = programId;
    this.target = channel;
    const epoch = ++this.epoch;
    const old = this.state.tune;
    this.detach();
    this.publish({
      tune: null,
      loading: true,
      error: "",
      message: "",
      needsPlay: false,
    });
    try {
      const result = await playbackAPI.tune(
        channel,
        mode,
        this.client,
        { ...capabilities(this.video), forceTranscode: this.force },
        this.csrf,
        old?.id,
        programId,
      );
      if (this.disposed || epoch !== this.epoch) {
        await playbackAPI.stop(result.id, this.csrf);
        return;
      }
      this.attach(result);
    } catch (e) {
      if (epoch === this.epoch)
        this.publish({
          loading: false,
          error: e instanceof Error ? e.message : "Playback could not start",
        });
    }
  }
  private attach(tune: Tune) {
    this.publish({ tune, error: "", message: "" });
    this.metadataAt = performance.now();
    if (tune.delivery === "METADATA_PRESENTATION") {
      this.ready = true;
      this.publish({ loading: false, needsPlay: false });
      return;
    }
    const video = this.video;
    const offset = tune.positionMs / 1000;
    const attachedAt = performance.now();
    const seek = () => {
      if (this.state.tune?.streamUrl !== tune.streamUrl) return;
      try {
        video.currentTime = Math.min(
          offset +
            (tune.mode === "LIVE"
              ? (performance.now() - attachedAt) / 1000
              : 0),
          Math.max(0, video.duration - 0.25),
        );
      } catch {
        this.publish({ message: "Seeking to live position…" });
      }
      void this.play();
    };
    video.addEventListener("loadedmetadata", seek, { once: true });
    if (tune.mime.includes("mpegurl") && Hls.isSupported()) {
      const hls = new Hls({
        startPosition: offset,
        maxBufferLength: 30,
        maxMaxBufferLength: 60,
      });
      this.hls = hls;
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) void this.failed();
      });
      hls.loadSource(tune.streamUrl);
      hls.attachMedia(video);
    } else {
      video.src = tune.streamUrl;
      video.load();
    }
  }
  async play() {
    try {
      await this.video.play();
    } catch (e) {
      if (e instanceof DOMException && e.name === "NotAllowedError")
        this.publish({ needsPlay: true, loading: false });
      else if (e instanceof DOMException && e.name === "AbortError") return;
      else
        this.publish({
          error:
            "Playback could not start. Retry or choose a compatible stream.",
          loading: false,
        });
    }
  }
  toggle() {
    if (this.state.tune?.delivery === "METADATA_PRESENTATION") return;
    if (this.video.paused) void this.play();
    else this.video.pause();
  }
  async heartbeat(ended = false) {
    this.ended = this.ended || ended;
    ended = this.ended;
    const t = this.state.tune;
    if (!t || !this.ready || this.polling || this.disposed) return;
    this.polling = true;
    try {
      const metadata = t.delivery === "METADATA_PRESENTATION";
      const position = metadata
        ? Math.min(
            t.durationMs - 1,
            t.positionMs + performance.now() - this.metadataAt,
          )
        : Math.round(this.video.currentTime * 1000);
      const u = await playbackAPI.heartbeat(
        t.id,
        Math.round(position),
        !metadata && this.video.paused && !ended ? "PAUSED" : "PLAYING",
        ended,
        this.csrf,
      );
      if (this.state.tune?.streamUrl !== t.streamUrl) return;
      this.publish({
        tune: {
          ...t,
          mode: u.mode,
          positionMs: Math.round(position),
          liveOffsetMs: u.liveOffsetMs,
          serverTime: u.serverTime,
        },
        message: u.message ?? this.state.message,
      });
      if (metadata) this.metadataAt = performance.now();
      if (u.transition) {
        await this.advance();
      } else if (ended) {
        this.publish({
          loading: false,
          message:
            u.message || "Program finished. Go Live to rejoin the channel.",
        });
      }
    } catch (e) {
      if (this.state.tune?.id === t.id)
        this.publish({
          error: e instanceof Error ? e.message : "Connection interrupted",
          loading: false,
        });
    } finally {
      this.polling = false;
    }
  }
  async advance(fallback = false) {
    const t = this.state.tune;
    if (!t) return;
    const epoch = ++this.epoch;
    this.detach();
    this.publish({ loading: true, error: "" });
    try {
      const next = await playbackAPI.advance(t.id, this.csrf, fallback);
      if (!this.disposed && epoch === this.epoch) this.attach(next);
    } catch (e) {
      this.publish({
        loading: false,
        error:
          e instanceof Error ? e.message : "Transition failed. Retry playback.",
      });
    }
  }
  async goLive() {
    this.targetProgram = undefined;
    const t = this.state.tune;
    if (!t) return this.tune(this.target);
    const epoch = ++this.epoch;
    this.detach();
    this.publish({ loading: true, error: "", message: "" });
    try {
      const result = await playbackAPI.goLive(t.id, this.csrf);
      if (this.disposed || epoch !== this.epoch) {
        await playbackAPI.stop(result.id, this.csrf);
        return;
      }
      this.attach(result);
    } catch (e) {
      this.publish({
        loading: false,
        error: e instanceof Error ? e.message : "Could not rejoin live",
      });
    }
  }
  async failed() {
    if (this.disposed || !this.state.tune) return;
    if (this.state.tune.program.entryType === "INTERSTITIAL") {
      await this.advance(true);
      return;
    }
    if (this.state.tune.delivery === "DIRECT_PLAY" && !this.force) {
      this.force = true;
      await this.tune(
        this.target,
        this.state.tune.mode === "FROM_BEGINNING" ? "FROM_BEGINNING" : "LIVE",
        this.targetProgram,
      );
      return;
    }
    this.publish({
      loading: false,
      error:
        "The stream was interrupted or Plex could not convert this format. Retry playback.",
    });
  }
  retry() {
    return this.tune(
      this.target,
      this.targetProgram ? "FROM_BEGINNING" : "LIVE",
      this.targetProgram,
    );
  }
  dispose() {
    this.disposed = true;
    this.epoch++;
    clearInterval(this.timer);
    for (const [name, fn] of this.listeners)
      this.video.removeEventListener(name, fn);
    this.detach();
    const t = this.state.tune;
    if (t)
      void playbackAPI.stop(t.id, this.csrf).catch(() => {
        /* Server expires abandoned sessions within 90 seconds. */
      });
  }
}
