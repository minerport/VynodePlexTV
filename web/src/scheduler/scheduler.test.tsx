import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ScheduleAdmin } from "./ScheduleAdmin";
import { scheduleAPI, localDay, shiftDay, displayTime } from "./api";
const settings = {
  channelId: "c",
  mode: "SEQUENTIAL",
  timezone: "America/New_York",
  anchorMs: Date.parse("2026-10-10T00:00:00Z"),
  seed: "fixed",
  movieRepeatDays: 14,
  episodeRepeatDays: 30,
  pastDays: 7,
  futureDays: 14,
  includeUnavailable: false,
  enabled: true,
};
const status = {
  channelId: "c",
  name: "Cinema",
  number: "101",
  mode: "SEQUENTIAL",
  timezone: settings.timezone,
  status: "READY",
  resolved: 5,
  schedulable: 5,
  excluded: 0,
  coverageStart: settings.anchorMs,
  coverageEnd: settings.anchorMs + 2000000000,
  programCount: 300,
  lastGenerated: settings.anchorMs,
  lastError: "",
  integrity: [],
  warnings: [],
  revision: 1,
};
function mocks() {
  vi.spyOn(scheduleAPI, "list").mockResolvedValue([status]);
  vi.spyOn(scheduleAPI, "settings").mockResolvedValue(settings);
  vi.spyOn(scheduleAPI, "status").mockResolvedValue(status);
  vi.spyOn(scheduleAPI, "programs").mockResolvedValue({
    items: [
      {
        id: "p",
        mediaId: "m",
        start: settings.anchorMs,
        end: settings.anchorMs + 3600000,
        durationMs: 3600000,
        title: "Episode title",
        showTitle: "The show",
        season: 3,
        episode: 12,
        available: true,
        artwork: "/poster",
        policyReason: "sequential order",
      },
    ],
    more: false,
    timezone: settings.timezone,
  });
  vi.spyOn(scheduleAPI, "jobs").mockResolvedValue([
    {
      id: "j",
      type: "SCHEDULE_GENERATE",
      status: "RUNNING",
      processed: 0,
      repaired: 0,
      error: "",
      createdAt: 1,
    },
  ]);
  vi.spyOn(scheduleAPI, "save").mockResolvedValue(settings);
  vi.spyOn(scheduleAPI, "run").mockResolvedValue({ jobId: "j" });
}
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
describe("schedule administration", () => {
  it("shows saved timelines, episode context, timezone and honest job progress", async () => {
    mocks();
    render(<ScheduleAdmin csrf="csrf" initialId="c" onClose={() => {}} />);
    expect(await screen.findByText("The show")).toBeVisible();
    expect(screen.getByText(/S03 E12/)).toBeVisible();
    expect(screen.getByText(/Integrity: OK/)).toBeVisible();
    fireEvent.click(screen.getByText("Scheduler jobs"));
    expect(screen.getByText(/Preparing timeline/)).toBeVisible();
    fireEvent.click(screen.getByText("Next day"));
    await waitFor(() => expect(scheduleAPI.programs).toHaveBeenCalledTimes(2));
  });
  it("saves settings and confirms destructive regeneration", async () => {
    mocks();
    render(<ScheduleAdmin csrf="csrf" initialId="c" onClose={() => {}} />);
    await screen.findByText("The show");
    fireEvent.click(screen.getByText("Scheduling settings"));
    fireEvent.change(screen.getByLabelText("Scheduling mode"), {
      target: { value: "SMART_RANDOM" },
    });
    expect(screen.getByLabelText("Movie repeat window (days)")).toBeVisible();
    fireEvent.click(screen.getByText("Save schedule settings"));
    await waitFor(() => expect(scheduleAPI.save).toHaveBeenCalled());
    fireEvent.click(screen.getByText("Regenerate future schedule"));
    expect(scheduleAPI.run).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Confirm regeneration"));
    await waitFor(() =>
      expect(scheduleAPI.run).toHaveBeenCalledWith("c", "regenerate", "csrf"),
    );
  });
  it("lists channels and reports empty days", async () => {
    mocks();
    vi.mocked(scheduleAPI.programs).mockResolvedValue({
      items: [],
      more: false,
      timezone: "UTC",
    });
    render(<ScheduleAdmin csrf="csrf" onClose={() => {}} />);
    await screen.findByText("Cinema");
    fireEvent.click(screen.getByText("Open schedule"));
    expect(await screen.findByText(/No programs for this day/)).toBeVisible();
  });
  it("surfaces errors and backs out by keyboard", async () => {
    mocks();
    vi.mocked(scheduleAPI.list).mockRejectedValue(new Error("Offline"));
    const close = vi.fn();
    render(<ScheduleAdmin csrf="csrf" onClose={close} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("Offline");
    fireEvent.keyDown(window, { key: "Escape" });
    expect(close).toHaveBeenCalled();
  });
  it("uses calendar days and configured timezone over DST", () => {
    expect(shiftDay("2026-03-08", 1)).toBe("2026-03-09");
    expect(localDay("America/New_York", new Date("2026-10-17T02:00:00Z"))).toBe(
      "2026-10-16",
    );
    expect(
      displayTime(Date.parse("2026-11-01T05:30:00Z"), "America/New_York"),
    ).toContain("1:30");
  });
});
