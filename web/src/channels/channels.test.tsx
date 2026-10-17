import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useState } from "react";
import {
  channelAPI,
  moveItem,
  newSource,
  newDefinition,
  type Rule,
  type Field,
} from "./api";
import { RuleBuilder } from "./RuleBuilder";
import { MediaPicker } from "./MediaPicker";
import { Preview } from "./Preview";
import { ChannelEditor } from "./ChannelEditor";
import { ChannelsPage } from "./ChannelsPage";
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});
it("edits Recently Added lookback without exposing a contradictory rule", async () => {
  vi.spyOn(channelAPI, "preview").mockResolvedValue({
    count: 0,
    items: [],
    offset: 0,
    limit: 24,
    warnings: [],
  });
  vi.spyOn(channelAPI, "save").mockRejectedValue(new Error("saved test"));
  const d = newDefinition("107");
  d.name = "New episodes";
  d.source = { ...newSource("RULE"), recent: { kind: "TV", days: 30 } };
  render(
    <ChannelEditor
      initial={d}
      groups={[]}
      fields={fields}
      csrf="test"
      onSaved={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  fireEvent.change(screen.getByLabelText(/Lookback window/), {
    target: { value: "43" },
  });
  expect(screen.queryByLabelText("Group logic")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Save channel" }));
  await waitFor(() =>
    expect(channelAPI.save).toHaveBeenCalledWith(
      undefined,
      expect.objectContaining({
        source: expect.objectContaining({ recent: { kind: "TV", days: 43 } }),
      }),
      "test",
    ),
  );
});
const fields: Field[] = [
  {
    name: "media_type",
    label: "Media type",
    kind: "enum",
    operators: ["is", "in"],
  },
  {
    name: "year",
    label: "Year",
    kind: "number",
    operators: ["equals", "between"],
  },
  {
    name: "availability",
    label: "Available",
    kind: "boolean",
    operators: ["is_true", "is_false"],
  },
];
it("builds nested NOT and numeric ranges without JSON editing", () => {
  vi.spyOn(channelAPI, "lookup").mockResolvedValue({
    id: "movie",
    name: "Movie",
    context: "",
    count: 0,
    artworkId: "",
    seasons: 0,
    episodes: 0,
  });
  let result: Rule | undefined;
  function Harness() {
    const [rule, setRule] = useState(newSource("RULE").rule!);
    result = rule;
    return <RuleBuilder rule={rule} fields={fields} onChange={setRule} />;
  }
  render(<Harness />);
  fireEvent.click(screen.getByText("+ Add group"));
  fireEvent.change(screen.getAllByLabelText("Group logic")[1], {
    target: { value: "NOT" },
  });
  fireEvent.change(screen.getAllByLabelText("Field")[1], {
    target: { value: "year" },
  });
  fireEvent.change(screen.getAllByLabelText("Operator")[1], {
    target: { value: "between" },
  });
  fireEvent.change(screen.getByLabelText("From"), {
    target: { value: "1990" },
  });
  fireEvent.change(screen.getByLabelText("To"), { target: { value: "1999" } });
  expect(result?.root.children?.[1]).toEqual({
    type: "group",
    operator: "NOT",
    children: [
      {
        type: "condition",
        field: "year",
        operator: "between",
        value: [1990, 1999],
      },
    ],
  });
});
it("preserves static order when moving selected media", () => {
  expect(moveItem(["a", "b", "c"], 2, 0)).toEqual(["c", "a", "b"]);
  expect(newSource("SHOW").show?.excludeUnavailable).toBe(true);
});
it("selects a page and retains unavailable selected media", async () => {
  vi.spyOn(channelAPI, "preview").mockImplementation(async (source) => ({
    count: source.type === "STATIC" ? 1 : 2,
    items: (source.type === "STATIC" ? ["old"] : ["a", "b"]).map((id) => ({
      id,
      title: id,
      type: "movie",
      year: 2000,
      showTitle: "",
      season: null,
      episode: null,
      durationMs: 60000,
      library: "Movies",
      available: id !== "old",
      eligible: id !== "old",
    })),
    offset: 0,
    limit: 24,
    warnings: [],
  }));
  const change = vi.fn();
  render(<MediaPicker ids={["old"]} csrf="test" onChange={change} />);
  await screen.findByText("1. old · Unavailable / inactive");
  await waitFor(() =>
    expect(screen.getAllByRole("button", { name: /Select$/ })).toHaveLength(2),
  );
  fireEvent.click(screen.getByText("Select this page"));
  expect(change).toHaveBeenCalledWith(["old", "a", "b"]);
  fireEvent.click(screen.getByText("Clear selection"));
  expect(change).toHaveBeenCalledWith([]);
});
it("shows honest zero-match warning and server validation errors", async () => {
  vi.spyOn(channelAPI, "preview").mockResolvedValue({
    count: 0,
    items: [],
    offset: 0,
    limit: 24,
    warnings: [],
  });
  render(<Preview source={newSource("STATIC")} csrf="test" />);
  expect(
    await screen.findByText(/No eligible media matches/),
  ).toBeInTheDocument();
});
it("saves static source and surfaces duplicate number errors", async () => {
  vi.spyOn(channelAPI, "preview").mockResolvedValue({
    count: 0,
    items: [],
    offset: 0,
    limit: 24,
    warnings: [],
  });
  vi.spyOn(channelAPI, "save").mockRejectedValue(
    new Error("Channel number is already in use"),
  );
  const d = newDefinition("101");
  d.name = "Cinema";
  render(
    <ChannelEditor
      initial={d}
      groups={[]}
      fields={fields}
      csrf="test"
      onSaved={vi.fn()}
      onClose={vi.fn()}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Save channel" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Channel number is already in use",
  );
  expect(channelAPI.save).toHaveBeenCalledWith(undefined, d, "test");
});
it("does not fetch administration for regular users", () => {
  const list = vi.spyOn(channelAPI, "list");
  render(
    <ChannelsPage
      session={{
        user: { id: "u", username: "viewer", role: "USER" },
        csrfToken: "test",
        expiresAt: 0,
      }}
    />,
  );
  expect(screen.getByText(/An administrator manages/)).toBeInTheDocument();
  expect(list).not.toHaveBeenCalled();
});
