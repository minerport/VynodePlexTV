import { useEffect, useState } from "react";
import { channelAPI, errorText, type Option } from "./api";
export function MetadataPicker({
  field,
  label,
  values,
  onChange,
  multiple = false,
  parent = "",
}: {
  field: string;
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  multiple?: boolean;
  parent?: string;
}) {
  const [search, setSearch] = useState("");
  const [offset, setOffset] = useState(0);
  const [options, setOptions] = useState<Option[]>([]);
  const [selected, setSelected] = useState<Option[]>([]);
  const [more, setMore] = useState(false);
  const [error, setError] = useState("");
  const [open, setOpen] = useState(false);
  const key = values.join("\n");
  useEffect(() => {
    let active = true;
    const ids = key ? key.split("\n") : [];
    Promise.all(ids.map((id) => channelAPI.lookup(field, id)))
      .then((v) => {
        if (active) setSelected(v);
      })
      .catch((e) => {
        if (active) setError(errorText(e));
      });
    return () => {
      active = false;
    };
  }, [field, key]);
  useEffect(() => {
    if (!open) return;
    let active = true;
    const timer = window.setTimeout(() => {
      channelAPI
        .metadata(field, search, offset, parent)
        .then((v) => {
          if (active) {
            setOptions(v.items);
            setMore(v.more);
            setError("");
          }
        })
        .catch((e) => {
          if (active) setError(errorText(e));
        });
    }, 180);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [field, search, offset, parent, open]);
  return (
    <div className="metadata-picker">
      <div className="field-caption">{label}</div>
      <div className="selection-chips">
        {selected.map((o) => (
          <span key={o.id} title={o.context}>
            {o.name}
            <button
              type="button"
              aria-label={`Remove ${o.name}`}
              onClick={() => onChange(values.filter((v) => v !== o.id))}
            >
              ×
            </button>
          </span>
        ))}
      </div>
      <button
        type="button"
        className="secondary"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
      >
        {values.length
          ? `Change ${label.toLowerCase()}`
          : `Choose ${label.toLowerCase()}`}
      </button>
      {open && (
        <div className="metadata-results">
          <label>
            Search {label.toLowerCase()}
            <input
              value={search}
              onChange={(e) => {
                setSearch(e.target.value);
                setOffset(0);
              }}
            />
          </label>
          {options.map((o) => (
            <button
              className="metadata-option"
              type="button"
              key={o.id}
              aria-pressed={values.includes(o.id)}
              onClick={() => {
                onChange(multiple ? [...new Set([...values, o.id])] : [o.id]);
                if (!multiple) setOpen(false);
              }}
            >
              {o.artworkId && (
                <img
                  src={`/api/v1/media/${o.artworkId}/artwork/poster`}
                  alt=""
                  loading="lazy"
                  onError={(e) => {
                    e.currentTarget.hidden = true;
                  }}
                />
              )}
              <span>
                <strong>{o.name}</strong>
                <small>{o.context}</small>
                {field === "show" ? (
                  <small>
                    {o.seasons} seasons · {o.episodes} episodes
                  </small>
                ) : (
                  <small>{o.count} linked items</small>
                )}
              </span>
            </button>
          ))}
          {options.length === 0 && <p>No matching values.</p>}
          <div className="channel-actions">
            <button
              type="button"
              disabled={!offset}
              onClick={() => setOffset(Math.max(0, offset - 20))}
            >
              Previous values
            </button>
            <button
              type="button"
              disabled={!more}
              onClick={() => setOffset(offset + 20)}
            >
              More values
            </button>
          </div>
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
