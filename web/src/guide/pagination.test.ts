import { expect, it, vi, afterEach } from "vitest";
import { request } from "../api";
import { guideAPI } from "./api";
vi.mock("../api", () => ({ request: vi.fn() }));
afterEach(() => vi.clearAllMocks());
it.each(["now", "soon"] as const)(
  "paginates %s through every channel instead of truncating at 100",
  async (kind) => {
    vi.mocked(request)
      .mockResolvedValueOnce({
        channels: Array.from({ length: 100 }, (_, i) => ({ id: String(i) })),
        warnings: [],
        more: true,
      })
      .mockResolvedValueOnce({
        channels: [{ id: "last" }],
        warnings: [],
        more: false,
      });
    const data = await guideAPI[kind]();
    expect(data.channels).toHaveLength(101);
    expect(data.channels.at(-1)?.id).toBe("last");
    expect(request).toHaveBeenLastCalledWith(
      expect.stringContaining("offset=100"),
    );
  },
);
