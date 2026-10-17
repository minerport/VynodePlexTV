import { useEffect, useState } from "react";
export const MINUTE = 60000,
  HOUR = 60 * MINUTE,
  SCALE = 6,
  ROW = 110;
export const position = (time: number, start: number) =>
  ((time - start) / MINUTE) * SCALE;
export function geometry(start: number, end: number, from: number, to: number) {
  return {
    left: position(Math.max(start, from), from),
    width: Math.max(1, position(Math.min(end, to), Math.max(start, from))),
  };
}
export const stateAt = (p: { start: number; end: number }, now: number) =>
  p.end <= now ? "PAST" : p.start <= now ? "CURRENT" : "FUTURE";
export const timeLabel = (n: number, zone: string) =>
  new Date(n).toLocaleTimeString("en-US", {
    timeZone: zone,
    hour: "numeric",
    minute: "2-digit",
  });
export const dayLabel = (n: number, zone: string) =>
  new Date(n).toLocaleDateString("en-US", {
    timeZone: zone,
    weekday: "short",
    month: "short",
    day: "numeric",
  });
// Resolve calendar midnight in an IANA zone, allowing 23/25-hour DST days.
export function dayStart(at: number, zone: string, delta = 0) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone: zone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(at);
  const value = (key: string) =>
    Number(parts.find((p) => p.type === key)?.value);
  const date = new Date(
    Date.UTC(value("year"), value("month") - 1, value("day") + delta),
  );
  const target = date.getTime();
  let guess = target;
  for (let i = 0; i < 4; i++) {
    const p = new Intl.DateTimeFormat("en-CA", {
      timeZone: zone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hourCycle: "h23",
    }).formatToParts(guess);
    const v = (k: string) => Number(p.find((x) => x.type === k)?.value);
    const local = Date.UTC(
      v("year"),
      v("month") - 1,
      v("day"),
      v("hour"),
      v("minute"),
      v("second"),
    );
    guess += target - local;
  }
  return guess;
}
export function useServerClock(server: number) {
  const [clock, setClock] = useState(() => ({ base: server, time: server }));
  useEffect(() => {
    const at = performance.now();
    const update = () =>
      setClock({ base: server, time: server + performance.now() - at });
    const first = setTimeout(update, 0);
    const timer = setInterval(update, 1000);
    return () => {
      clearTimeout(first);
      clearInterval(timer);
    };
  }, [server]);
  return clock.base === server ? clock.time : server;
}
