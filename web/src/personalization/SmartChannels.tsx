import { useEffect, useState } from "react";
import { request } from "../api";
import { channelAPI, type Option } from "../channels/api";
interface Template {
  id: string;
  name: string;
  description: string;
  field: string;
}
export function SmartChannels({
  csrf,
  onClose,
  onCreated,
}: {
  csrf: string;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [templates, setTemplates] = useState<Template[]>([]),
    [selected, setSelected] = useState<Template | null>(null),
    [options, setOptions] = useState<Option[]>([]),
    [search, setSearch] = useState(""),
    [selection, setSelection] = useState(""),
    [name, setName] = useState(""),
    [number, setNumber] = useState(""),
    [decade, setDecade] = useState(1990),
    [mediaType, setMediaType] = useState(""),
    [start, setStart] = useState(""),
    [end, setEnd] = useState(""),
    [zone, setZone] = useState("America/New_York"),
    [preview, setPreview] = useState<{
      count: number;
      items: { id: string; title: string }[];
    } | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    void request<{ items: Template[] }>("/smart-channel-templates")
      .then((v) => setTemplates(v.items))
      .catch((e) => setError(e.message));
    void request<{ timezone: string }>("/settings")
      .then((v) => setZone(v.timezone))
      .catch((e) => setError(e.message));
  }, []);
  useEffect(() => {
    let live = true;
    const t = setTimeout(() => {
      if (selected && selected.field !== "year")
        void channelAPI
          .metadata(selected.field, search)
          .then((v) => {
            if (live) setOptions(v.items);
          })
          .catch((e) => {
            if (live) setError(e.message);
          });
    }, 200);
    return () => {
      live = false;
      clearTimeout(t);
    };
  }, [selected, search]);
  const body = {
    template: selected?.id,
    selection,
    name,
    number,
    decade,
    mediaType,
    startDate: start,
    endDate: end,
    timezone: zone,
  };
  async function run(create: boolean) {
    setBusy(true);
    setError("");
    try {
      if (create) {
        await request(
          "/smart-channel-templates/instantiate",
          "POST",
          body,
          csrf,
        );
        onCreated();
      } else {
        const v = await request<{
          preview: { count: number; items: { id: string; title: string }[] };
        }>("/smart-channel-templates/preview", "POST", body, csrf);
        setPreview(v.preview);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to create channel");
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="my-channels">
      <button onClick={onClose}>← Channels</button>
      <p className="eyebrow">EXPLICIT PROGRAMMING</p>
      <h1>Create Smart Channel</h1>
      <p>
        These templates create shared channels. Everyone sees the same persisted
        broadcast.
      </p>
      {error && <p role="alert">{error}</p>}
      <div className="personal-toolbar">
        {templates.map((t) => (
          <button
            key={t.id}
            aria-pressed={selected?.id === t.id}
            onClick={() => {
              setSelected(t);
              setSelection("");
              setSearch("");
              setOptions([]);
              setPreview(null);
              setName(t.name);
            }}
          >
            {t.name}
          </button>
        ))}
      </div>
      {selected && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void run(false);
          }}
          onChange={() => setPreview(null)}
        >
          <p>{selected.description}</p>
          <label>
            Name
            <input
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label>
            Channel number (blank chooses next available)
            <input value={number} onChange={(e) => setNumber(e.target.value)} />
          </label>
          {selected.field === "year" ? (
            <label>
              Decade
              <select
                value={decade}
                onChange={(e) => setDecade(Number(e.target.value))}
              >
                {[1980, 1990, 2000, 2010, 2020].map((y) => (
                  <option key={y} value={y}>
                    {y}s
                  </option>
                ))}
              </select>
            </label>
          ) : (
            <>
              <label>
                Search {selected.field}
                <input
                  value={search}
                  onChange={(e) => {
                    setSearch(e.target.value);
                    setSelection("");
                  }}
                />
              </label>
              <label>
                Choose {selected.field}
                <select
                  required
                  value={selection}
                  onChange={(e) => {
                    setSelection(e.target.value);
                    setName(
                      options.find((o) => o.id === e.target.value)?.name ??
                        selected.name,
                    );
                  }}
                >
                  <option value="">Select synchronized metadata</option>
                  {options.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name} · {o.context}
                    </option>
                  ))}
                </select>
              </label>
            </>
          )}
          <label>
            Media
            <select
              value={mediaType}
              onChange={(e) => setMediaType(e.target.value)}
            >
              <option value="">Movies and episodes</option>
              <option value="movie">Movies</option>
              <option value="episode">Episodes</option>
            </select>
          </label>
          {selected.id === "seasonal" && (
            <>
              <label>
                First active date
                <input
                  required
                  type="date"
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                />
              </label>
              <label>
                Last active date
                <input
                  required
                  type="date"
                  value={end}
                  onChange={(e) => setEnd(e.target.value)}
                />
              </label>
              <label>
                Timezone
                <input
                  required
                  value={zone}
                  onChange={(e) => setZone(e.target.value)}
                />
              </label>
              <p>
                Explicit dates, including year. Programs already started may
                finish after the window closes.
              </p>
            </>
          )}
          <button disabled={busy} type="submit">
            Preview membership
          </button>
        </form>
      )}
      {preview && (
        <div>
          <h2>{preview.count} matching items</h2>
          <p>
            Smart Random · Existing presentation defaults · Editable after
            creation
          </p>
          <ul>
            {preview.items.map((m) => (
              <li key={m.id}>{m.title}</li>
            ))}
          </ul>
          <button
            className="primary"
            disabled={busy}
            onClick={() => void run(true)}
          >
            Create shared channel
          </button>
        </div>
      )}
    </section>
  );
}
