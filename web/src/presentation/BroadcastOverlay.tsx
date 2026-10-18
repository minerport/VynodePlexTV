import type { Tune } from "../playback/api";
import "./presentation.css";
export type BroadcastContext = Pick<
  Tune,
  | "channel"
  | "origin"
  | "mode"
  | "state"
  | "serverTime"
  | "delivery"
  | "branding"
> & {
  program: Pick<Tune["program"], "end">;
  next: Pick<
    NonNullable<Tune["next"]>,
    "title" | "start" | "showTitle" | "season" | "episode"
  > | null;
};
export function BroadcastOverlay({
  tune: t,
  controlsVisible = false,
  previewLogo,
}: {
  tune: BroadcastContext;
  controlsVisible?: boolean;
  previewLogo?: string;
}) {
  const b = t.branding,
    metadata = t.delivery === "METADATA_PRESENTATION";
  const nextUp =
    !!b &&
    t.mode === "LIVE" &&
    t.origin === "LIVE" &&
    t.state !== "PAUSED" &&
    t.program.end - t.serverTime <= b.nextUpSeconds * 1000 &&
    t.program.end > t.serverTime;
  return (
    <>
      {b?.bug && !controlsVisible && (
        <img
          alt={`${t.channel.name} channel logo`}
          className={`channel-bug ${b.corner}`}
          src={
            previewLogo ||
            (b.compactLogoId
              ? `/api/v1/watch/channels/${t.channel.id}/artwork/compact`
              : t.channel.logo)
          }
          style={{ opacity: b.opacity, width: b.size }}
        />
      )}
      {(metadata || nextUp) && (
        <section
          className={metadata ? "station-break" : "coming-next"}
          aria-label="Coming Up Next"
        >
          <p>{metadata ? "STATION BREAK" : "COMING UP NEXT"}</p>
          <h2>{t.next?.showTitle || t.next?.title || t.channel.name}</h2>
          {t.next?.showTitle && (
            <p>
              S{t.next.season} E{t.next.episode} · {t.next.title}
            </p>
          )}
          <span>
            {t.next
              ? `Starts in ${Math.max(0, Math.ceil((t.next.start - t.serverTime) / 1000))} seconds`
              : "Programming resumes shortly"}
          </span>
        </section>
      )}
    </>
  );
}
