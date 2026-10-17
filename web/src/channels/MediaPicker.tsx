import { useEffect, useMemo, useState } from "react";
import {
  channelAPI,
  condition,
  duration,
  errorText,
  moveItem,
  type Media,
  type RuleNode,
  type Source,
} from "./api";
import { MetadataPicker } from "./MetadataPicker";
export function MediaPicker({
  ids,
  onChange,
  csrf,
  single = false,
}: {
  ids: string[];
  onChange: (ids: string[]) => void;
  csrf: string;
  single?: boolean;
}) {
  const [q, setQ] = useState("");
  const [type, setType] = useState("");
  const [library, setLibrary] = useState<string[]>([]);
  const [genre, setGenre] = useState<string[]>([]);
  const [show, setShow] = useState<string[]>([]);
  const [offset, setOffset] = useState(0);
  const [selectedPage, setSelectedPage] = useState(0);
  const [items, setItems] = useState<Media[]>([]);
  const [selected, setSelected] = useState<Media[]>([]);
  const [count, setCount] = useState(0);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [includeUnavailable, setIncludeUnavailable] = useState(false);
  const [drag, setDrag] = useState<number | null>(null);
  const source = useMemo<Source>(() => {
    const children: RuleNode[] = [
      condition("media_type", "in", type ? [type] : ["movie", "episode"]),
    ];
    if (q) children.push(condition("title", "contains", q));
    for (const [field, values] of [
      ["library", library],
      ["genre", genre],
      ["show", show],
    ] as const)
      if (values.length) children.push(condition(field, "in", values));
    return {
      type: "RULE",
      rule: { version: 1, root: { type: "group", operator: "AND", children } },
    };
  }, [q, type, library, genre, show]);
  useEffect(() => {
    let live = true;
    const timer = setTimeout(() => {
      setBusy(true);
      channelAPI
        .preview(source, csrf, offset, includeUnavailable)
        .then((r) => {
          if (live) {
            setItems(r.items);
            setCount(r.count);
            setError("");
          }
        })
        .catch((e) => {
          if (live) setError(errorText(e));
        })
        .finally(() => {
          if (live) setBusy(false);
        });
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [source, csrf, offset, includeUnavailable]);
  useEffect(() => {
    let live = true;
    channelAPI
      .preview(
        { type: "STATIC", static: { mediaIds: ids } },
        csrf,
        selectedPage,
        true,
      )
      .then((r) => {
        if (live) setSelected(r.items);
      })
      .catch((e) => {
        if (live) setError(errorText(e));
      });
    return () => {
      live = false;
    };
  }, [ids, csrf, selectedPage]);
  const toggle = (id: string) =>
    onChange(
      single
        ? [id]
        : ids.includes(id)
          ? ids.filter((x) => x !== id)
          : [...ids, id],
    );
  return (
    <section className="media-picker">
      <div className="channel-filters">
        <label>
          Search title
          <input
            value={q}
            onChange={(e) => {
              setQ(e.target.value);
              setOffset(0);
            }}
            placeholder="Find a movie or episode"
          />
        </label>
        <label>
          Media type
          <select
            value={type}
            onChange={(e) => {
              setType(e.target.value);
              setOffset(0);
            }}
          >
            <option value="">Movies and episodes</option>
            <option value="movie">Movies</option>
            <option value="episode">Episodes</option>
          </select>
        </label>
        <MetadataPicker
          field="library"
          label="Library"
          values={library}
          multiple
          onChange={(v) => {
            setLibrary(v);
            setOffset(0);
          }}
        />
        <MetadataPicker
          field="genre"
          label="Genre"
          values={genre}
          multiple
          onChange={(v) => {
            setGenre(v);
            setOffset(0);
          }}
        />
        <MetadataPicker
          field="show"
          label="Show"
          values={show}
          onChange={(v) => {
            setShow(v);
            setOffset(0);
          }}
        />
      </div>
      <label className="check-label">
        <input
          type="checkbox"
          checked={includeUnavailable}
          onChange={(e) => {
            setIncludeUnavailable(e.target.checked);
            setOffset(0);
          }}
        />
        Include unavailable inventory for inspection
      </label>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="channel-actions">
        <span role="status">
          {busy ? "Finding media…" : `${count.toLocaleString()} matching items`}
        </span>
        {!single && (
          <>
            <button
              type="button"
              onClick={() =>
                onChange([...new Set([...ids, ...items.map((m) => m.id)])])
              }
            >
              Select this page
            </button>
            <button type="button" onClick={() => onChange([])}>
              Clear selection
            </button>
          </>
        )}
      </div>
      <div className="channel-media-grid">
        {items.map((m) => (
          <button
            type="button"
            className="media-choice"
            key={m.id}
            aria-pressed={ids.includes(m.id)}
            onClick={() => toggle(m.id)}
          >
            <img
              loading="lazy"
              src={`/api/v1/media/${m.id}/artwork/poster`}
              alt=""
            />
            <strong>
              {m.showTitle ? `${m.showTitle} · ` : ""}
              {m.title}
            </strong>
            <small>
              {m.type === "episode"
                ? `S${m.season ?? "?"} E${m.episode ?? "?"}`
                : m.year}{" "}
              · {duration(m.durationMs)}
            </small>
            <small>
              {m.library}
              {!m.eligible ? " · Unavailable / inactive" : ""}
            </small>
            <span>{ids.includes(m.id) ? "Selected" : "Select"}</span>
          </button>
        ))}
      </div>
      <Pager offset={offset} count={count} onChange={setOffset} />
      <div className="selected-media">
        <h3>
          {ids.length} selected {single ? "" : "· ordered membership"}
        </h3>
        {selected.map((m) => {
          const i = ids.indexOf(m.id);
          return (
            <div
              className="selected-row"
              key={m.id}
              draggable={!single}
              onDragStart={() => setDrag(i)}
              onDragOver={(e) => e.preventDefault()}
              onDrop={(e) => {
                e.preventDefault();
                if (drag !== null) onChange(moveItem(ids, drag, i));
                setDrag(null);
              }}
            >
              <span>
                {i + 1}. {m.showTitle ? `${m.showTitle} · ` : ""}
                {m.title}
                {!m.eligible ? " · Unavailable / inactive" : ""}
              </span>
              {!single && (
                <>
                  <button
                    type="button"
                    aria-label={`Move ${m.title} up`}
                    disabled={i === 0}
                    onClick={() => onChange(moveItem(ids, i, i - 1))}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    aria-label={`Move ${m.title} down`}
                    disabled={i === ids.length - 1}
                    onClick={() => onChange(moveItem(ids, i, i + 1))}
                  >
                    ↓
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={() => onChange(ids.filter((id) => id !== m.id))}
              >
                Remove
              </button>
            </div>
          );
        })}
        <Pager
          offset={selectedPage}
          count={ids.length}
          onChange={setSelectedPage}
        />
      </div>
    </section>
  );
}
export function Pager({
  offset,
  count,
  onChange,
}: {
  offset: number;
  count: number;
  onChange: (v: number) => void;
}) {
  return count > 24 || offset > 0 ? (
    <div className="channel-actions">
      <button
        type="button"
        disabled={offset === 0}
        onClick={() => onChange(Math.max(0, offset - 24))}
      >
        Previous page
      </button>
      <span>
        {count
          ? `${offset + 1}–${Math.min(offset + 24, count)} of ${count}`
          : "No results"}
      </span>
      <button
        type="button"
        disabled={offset + 24 >= count}
        onClick={() => onChange(offset + 24)}
      >
        Next page
      </button>
    </div>
  ) : null;
}
