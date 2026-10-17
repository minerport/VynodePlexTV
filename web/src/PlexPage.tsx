import { useEffect, useRef, useState, type FormEvent } from "react";
import {
  ArrowLeft,
  Check,
  Film,
  Plus,
  Radio,
  RefreshCw,
  Search,
  Server,
  X,
} from "lucide-react";
import type { Session } from "./api";
import {
  plexAPI,
  displayTime,
  isActive,
  type Connection,
  type MediaItem,
  type MediaResult,
  type PlexServer,
  type PlexLibrary,
  type PlexJob,
  type PlexOptions,
} from "./plex-api";
import { usePlex } from "./usePlex";
import "./plex.css";
import { PlexSignIn } from "./PlexSignIn";

const errorMessage = (e: unknown) =>
  e instanceof Error ? e.message : "Unable to complete this request";
type Tab = "servers" | "libraries" | "media" | "jobs" | "preferences";

export function PlexPage({ session }: { session: Session }) {
  if (session.user.role === "USER")
    return (
      <section className="destination">
        <h1>Plex administration</h1>
        <p>Your administrator manages Plex connections and libraries.</p>
      </section>
    );
  return <PlexAdmin session={session} />;
}
function PlexAdmin({ session }: { session: Session }) {
  const { servers, libraries, jobs, error, loading, refresh } = usePlex();
  const [tab, setTab] = useState<Tab>("servers");
  const [editing, setEditing] = useState<PlexServer | null | undefined>();
  const [notice, setNotice] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const panel = useRef<HTMLDivElement>(null);
  const csrf = session.csrfToken;
  async function act(work: () => Promise<unknown>, success: string) {
    setBusy(true);
    setActionError("");
    setNotice("");
    try {
      await work();
      setNotice(success);
      await refresh();
    } catch (e) {
      setActionError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  function switchTab(next: Tab) {
    setTab(next);
    requestAnimationFrame(() => panel.current?.focus());
  }
  return (
    <section className="plex-page">
      <div className="plex-heading">
        <div>
          <span className="eyebrow">THE SOURCE OF IT ALL</span>
          <h1>Your library. Center stage.</h1>
          <p>
            Connect your Plex servers. Keep a dependable local inventory.
            <br />
            Your media stays with Plex.
          </p>
        </div>
        <span className="pill">
          <Radio size={15} /> PLEX INTEGRATION
        </span>
      </div>
      <div className="plex-metrics">
        <span>
          <strong>{servers.filter((s) => !s.disabled).length}</strong> servers
        </span>
        <span>
          <strong>{libraries.filter((l) => l.selected).length}</strong> selected
          libraries
        </span>
        <span>
          <strong>
            {libraries.reduce((n, l) => n + l.itemCount, 0).toLocaleString()}
          </strong>{" "}
          available items
        </span>
        <span>
          <strong>{jobs.filter(isActive).length}</strong> active jobs
        </span>
      </div>
      <div className="plex-tabs" role="navigation" aria-label="Plex sections">
        {(
          ["servers", "libraries", "media", "jobs", "preferences"] as Tab[]
        ).map((t) => (
          <button
            key={t}
            aria-current={tab === t ? "page" : undefined}
            onClick={() => switchTab(t)}
          >
            {t === "media"
              ? "Media browser"
              : t === "preferences"
                ? "Sync settings"
                : t[0].toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>
      {(error || actionError) && (
        <p className="error" role="alert">
          {actionError || error}
        </p>
      )}
      {notice && (
        <p className="success" role="status">
          {notice}
        </p>
      )}
      <div ref={panel} tabIndex={-1} className="plex-content">
        {loading ? (
          <p role="status">Loading Plex status…</p>
        ) : tab === "servers" ? (
          <>
            <div className="plex-toolbar">
              <h2>Plex servers</h2>
              <button className="primary" onClick={() => setEditing(null)}>
                <Plus size={17} />
                Add Plex server
              </button>
            </div>
            <PlexSignIn
              csrf={csrf}
              onConnected={async (server) => {
                await act(
                  () => plexAPI.action(server.id, "discover", csrf),
                  "Plex connected. Library discovery queued",
                );
              }}
            />
            {editing !== undefined && (
              <ConnectionForm
                server={editing}
                csrf={csrf}
                onClose={() => setEditing(undefined)}
                onSaved={async (server) => {
                  setEditing(undefined);
                  setNotice("Server connected. Discovering libraries…");
                  await act(
                    () => plexAPI.action(server.id, "discover", csrf),
                    "Library discovery queued",
                  );
                }}
              />
            )}
            {servers.length === 0 && editing === undefined ? (
              <div className="empty-panel">
                <Server size={38} />
                <h2>Bring your first library home.</h2>
                <p>
                  Add a Plex server using its URL and token. Credentials stay
                  encrypted on this server.
                </p>
                <button className="secondary" onClick={() => setEditing(null)}>
                  Connect Plex <Plus size={16} />
                </button>
              </div>
            ) : (
              <div className="server-grid">
                {servers.map((server) => (
                  <article className="server-card" key={server.id}>
                    <div className="server-card-top">
                      <span className="card-icon-static">
                        <Server />
                      </span>
                      <span
                        className={`connection-status ${server.status.toLowerCase()}`}
                      >
                        {server.disabled ? "DISCONNECTED" : server.status}
                      </span>
                    </div>
                    <h3>{server.name}</h3>
                    <p className="server-url">{server.baseUrl}</p>
                    <dl>
                      <div>
                        <dt>Server</dt>
                        <dd>
                          {server.friendlyName || server.machineIdentifier}
                        </dd>
                      </div>
                      <div>
                        <dt>Version / platform</dt>
                        <dd>
                          {server.version || "Unknown"} ·{" "}
                          {server.platform || "Unknown"}
                        </dd>
                      </div>
                      <div>
                        <dt>Last connected</dt>
                        <dd>{displayTime(server.lastSuccessAt)}</dd>
                      </div>
                      <div>
                        <dt>Libraries</dt>
                        <dd>
                          {
                            libraries.filter((l) => l.serverId === server.id)
                              .length
                          }
                        </dd>
                      </div>
                    </dl>
                    {server.lastError && (
                      <p className="error">{server.lastError}</p>
                    )}
                    <div className="card-actions">
                      <button
                        disabled={busy || server.disabled}
                        onClick={() =>
                          void act(
                            () => plexAPI.action(server.id, "test", csrf),
                            "Health check queued",
                          )
                        }
                      >
                        Test
                      </button>
                      <button
                        disabled={busy}
                        onClick={() => setEditing(server)}
                      >
                        Edit / reconnect
                      </button>
                      <button
                        disabled={busy || server.disabled}
                        onClick={() =>
                          void act(
                            () => plexAPI.action(server.id, "discover", csrf),
                            "Library discovery queued",
                          )
                        }
                      >
                        Refresh libraries
                      </button>
                      <button
                        disabled={
                          busy ||
                          server.disabled ||
                          !libraries.some(
                            (l) =>
                              l.serverId === server.id &&
                              l.selected &&
                              l.present,
                          )
                        }
                        onClick={() =>
                          void act(async () => {
                            for (const l of libraries.filter(
                              (l) =>
                                l.serverId === server.id &&
                                l.selected &&
                                l.present,
                            ))
                              await plexAPI.sync(l.id, "incremental", csrf);
                          }, "Selected libraries queued")
                        }
                      >
                        Sync selected
                      </button>
                      <button
                        disabled={busy || server.disabled}
                        className="quiet-danger"
                        onClick={() => {
                          if (
                            window.confirm(
                              "Disconnect this Plex server and erase its stored token? Local media rows will be retained; libraries will be deselected.",
                            )
                          )
                            void act(
                              () => plexAPI.remove(server.id, csrf),
                              "Server disconnected; inventory retained",
                            );
                        }}
                      >
                        Disconnect
                      </button>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </>
        ) : tab === "libraries" ? (
          <>
            <div className="plex-toolbar">
              <h2>Your libraries</h2>
              <button
                disabled={
                  busy || !libraries.some((l) => l.selected && l.present)
                }
                onClick={() =>
                  void act(async () => {
                    for (const l of libraries.filter(
                      (l) => l.selected && l.present,
                    ))
                      await plexAPI.sync(l.id, "incremental", csrf);
                  }, "Selected libraries queued")
                }
              >
                <RefreshCw size={16} />
                Sync selected
              </button>
            </div>
            <p className="plex-help">
              Select movie and TV libraries to synchronize. Deselecting
              preserves existing inventory. Other library types are shown for
              discovery only.
            </p>
            <div className="library-list">
              {libraries.length === 0 ? (
                <div className="empty-panel">
                  <h2>No libraries discovered</h2>
                  <p>Add a server, then refresh its libraries.</p>
                </div>
              ) : (
                libraries.map((l) => (
                  <LibraryRow
                    key={l.id}
                    library={l}
                    server={servers.find((s) => s.id === l.serverId)}
                    jobs={jobs}
                    busy={busy}
                    select={() =>
                      void act(
                        () => plexAPI.select(l.id, !l.selected, csrf),
                        "Library selection saved",
                      )
                    }
                    sync={(mode) =>
                      void act(
                        () => plexAPI.sync(l.id, mode, csrf),
                        "Synchronization queued",
                      )
                    }
                  />
                ))
              )}
            </div>
          </>
        ) : tab === "jobs" ? (
          <JobsPanel
            jobs={jobs}
            libraries={libraries}
            busy={busy}
            cancel={(id) =>
              void act(
                () => plexAPI.cancel(id, csrf),
                "Cancellation requested; inventory will be preserved",
              )
            }
          />
        ) : tab === "media" ? (
          <MediaBrowser servers={servers} libraries={libraries} />
        ) : (
          <SyncSettings csrf={csrf} />
        )}
      </div>
      <div className="info-strip">
        <Check size={17} />
        <span>
          Outages and interrupted scans never mark existing media missing.
          Inventory is reconciled only after a complete successful scan.
        </span>
      </div>
    </section>
  );
}

function ConnectionForm({
  server,
  csrf,
  onClose,
  onSaved,
}: {
  server: PlexServer | null;
  csrf: string;
  onClose: () => void;
  onSaved: (server: PlexServer) => Promise<void>;
}) {
  const [value, setValue] = useState<Connection>({
    baseUrl: server?.baseUrl ?? "",
    token: "",
    name: server?.name ?? "",
  });
  const [error, setError] = useState("");
  const [verified, setVerified] = useState("");
  const [busy, setBusy] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const saved = await plexAPI.save(value, server?.id, csrf);
      setValue({ ...value, token: "" });
      await onSaved(saved);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  async function test() {
    setBusy(true);
    setError("");
    setVerified("");
    try {
      const identity = await plexAPI.test(value, csrf);
      setVerified(
        `Connected to ${identity.friendlyName || "Plex Media Server"}`,
      );
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="connection-form" onSubmit={submit}>
      <div className="plex-toolbar">
        <h2>{server ? "Edit connection" : "Add Plex server"}</h2>
        <button
          className="icon-button"
          type="button"
          aria-label="Close connection form"
          disabled={busy}
          onClick={onClose}
        >
          <X />
        </button>
      </div>
      <div className="connection-fields">
        <label>
          Server URL
          <input
            autoFocus
            required
            type="text"
            placeholder="http://192.168.1.10:32400"
            value={value.baseUrl}
            onChange={(e) => {
              setValue({ ...value, baseUrl: e.target.value });
              setVerified("");
            }}
          />
        </label>
        <label>
          Display name (optional)
          <input
            maxLength={100}
            value={value.name}
            onChange={(e) => setValue({ ...value, name: e.target.value })}
          />
        </label>
        <label>
          Plex token{server ? " (leave blank to keep existing)" : ""}
          <input
            type="password"
            autoComplete="off"
            spellCheck={false}
            required={!server}
            value={value.token}
            onChange={(e) => {
              setValue({ ...value, token: e.target.value });
              setVerified("");
            }}
          />
        </label>
      </div>
      <p className="field-help">
        Use a token with access to the libraries you want to sync. It is
        encrypted locally and never included in server responses. HTTPS
        certificates are verified.
      </p>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {verified && (
        <p className="success" role="status">
          {verified}
        </p>
      )}
      <div className="card-actions">
        <button
          type="button"
          disabled={busy || !value.token || !value.baseUrl}
          onClick={() => void test()}
        >
          Test connection
        </button>
        <button className="primary" disabled={busy}>
          {busy ? "Connecting…" : server ? "Save connection" : "Connect server"}
        </button>
      </div>
    </form>
  );
}
function LibraryRow({
  library: l,
  server,
  jobs,
  busy,
  select,
  sync,
}: {
  library: PlexLibrary;
  server: PlexServer | undefined;
  jobs: PlexJob[];
  busy: boolean;
  select: () => void;
  sync: (mode: "full" | "incremental") => void;
}) {
  const active = jobs.find((j) => j.libraryId === l.id && isActive(j));
  return (
    <article className="library-row">
      <div className="library-main">
        <Film />
        <div>
          <h3>{l.title}</h3>
          <span>
            {server?.name} · {l.type}{" "}
            {!l.supported
              ? "· Discovery only"
              : !l.present
                ? "· No longer present"
                : ""}
          </span>
        </div>
      </div>
      <div className="library-summary">
        <strong>{l.itemCount.toLocaleString()} items</strong>
        <span>Last sync: {displayTime(l.lastSyncedAt)}</span>
      </div>
      <div className="card-actions">
        <button
          aria-pressed={l.selected}
          disabled={
            busy ||
            !!active ||
            !l.supported ||
            (!l.present && !l.selected) ||
            server?.disabled
          }
          onClick={select}
        >
          {l.selected ? "Selected" : "Select library"}
        </button>
        <button
          disabled={
            busy || !!active || !l.selected || !l.present || server?.disabled
          }
          onClick={() => sync("incremental")}
        >
          Sync changes
        </button>
        <button
          disabled={
            busy || !!active || !l.selected || !l.present || server?.disabled
          }
          onClick={() => sync("full")}
        >
          Full refresh
        </button>
      </div>
      {active && <JobProgress job={active} />}
    </article>
  );
}
export function JobProgress({ job }: { job: PlexJob }) {
  const known = job.total !== null && job.total > 0;
  return (
    <div className="job-progress">
      <div>
        <span>
          {job.status}
          {job.cancelRequested ? " · cancelling" : ""}
        </span>
        <span>
          {job.processed.toLocaleString()}
          {known
            ? ` / ${job.total!.toLocaleString()} items · ${Math.floor((job.processed / job.total!) * 100)}%`
            : " items processed"}
        </span>
      </div>
      {isActive(job) && (
        <progress
          aria-label="Synchronization progress"
          max={known ? job.total! : undefined}
          value={known ? job.processed : undefined}
        />
      )}
      <p>
        {job.currentItem ||
          (job.status === "QUEUED"
            ? "Waiting for the worker"
            : job.status === "RUNNING"
              ? "Contacting Plex…"
              : "")}
      </p>
      {job.error && <p className="error">{job.error}</p>}
    </div>
  );
}
function JobsPanel({
  jobs,
  libraries,
  busy,
  cancel,
}: {
  jobs: PlexJob[];
  libraries: PlexLibrary[];
  busy: boolean;
  cancel: (id: string) => void;
}) {
  return (
    <>
      <div className="plex-toolbar">
        <h2>Synchronization & jobs</h2>
        <span className="muted">Updates every four seconds</span>
      </div>
      {jobs.length === 0 ? (
        <div className="empty-panel">
          <h2>No jobs yet</h2>
          <p>
            Discovery, health checks, sync and cache maintenance appear here.
          </p>
        </div>
      ) : (
        <div className="job-list">
          {jobs.map((j) => (
            <article className="job-card" key={j.id}>
              <div className="plex-toolbar">
                <div>
                  <h3>
                    {libraries.find((l) => l.id === j.libraryId)?.title ??
                      j.type
                        .replaceAll("PLEX_", "")
                        .replaceAll("_", " ")
                        .toLowerCase()}
                  </h3>
                  <span>
                    {j.mode} · {displayTime(j.createdAt)}
                  </span>
                </div>
                {isActive(j) && (
                  <button
                    disabled={busy || j.cancelRequested}
                    onClick={() => cancel(j.id)}
                  >
                    Cancel job
                  </button>
                )}
              </div>
              <JobProgress job={j} />
            </article>
          ))}
        </div>
      )}
    </>
  );
}

function MediaBrowser({
  servers,
  libraries,
}: {
  servers: PlexServer[];
  libraries: PlexLibrary[];
}) {
  const [server, setServer] = useState("");
  const [library, setLibrary] = useState("");
  const [type, setType] = useState("");
  const [title, setTitle] = useState("");
  const [available, setAvailable] = useState("");
  const [trail, setTrail] = useState<MediaItem[]>([]);
  const [rootFilters, setRootFilters] = useState({ server: "", library: "" });
  const gridRef = useRef<HTMLDivElement>(null);
  const focusOnLoad = useRef(false);
  const parent = trail.at(-1)?.ratingKey ?? "";
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<MediaResult | null>(null);
  const [detail, setDetail] = useState<MediaItem | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    if (focusOnLoad.current && data) {
      const first = gridRef.current?.querySelector<HTMLButtonElement>("button");
      if (first) {
        first.focus();
        first.scrollIntoView?.({ block: "nearest" });
      }
      focusOnLoad.current = false;
    }
  }, [data]);
  useEffect(() => {
    let current = true;
    const timer = window.setTimeout(() => {
      const params = new URLSearchParams({
        server,
        library,
        type,
        q: title,
        available,
        parent,
        topLevel: String(!parent && !type),
        offset: String(offset),
        limit: "24",
      });
      plexAPI
        .media(params)
        .then((value) => {
          if (current) {
            setData(value);
            setError("");
          }
        })
        .catch((e) => {
          if (current) setError(errorMessage(e));
        });
    }, 200);
    return () => {
      current = false;
      window.clearTimeout(timer);
    };
  }, [server, library, type, title, available, parent, offset]);
  async function inspect(id: string) {
    try {
      setDetail(await plexAPI.item(id));
      setError("");
    } catch (e) {
      setError(errorMessage(e));
    }
  }
  function drill(item: MediaItem) {
    focusOnLoad.current = true;
    if (trail.length === 0) setRootFilters({ server, library });
    setServer(item.serverId);
    setLibrary(item.libraryId);
    setTrail([...trail, item]);
    setType(item.type === "show" ? "season" : "episode");
    setTitle("");
    setOffset(0);
    setDetail(null);
  }
  return (
    <>
      <div className="plex-toolbar">
        <h2>Library inventory</h2>
        <span className="muted">Metadata inspection · no playback</span>
      </div>
      <div className="media-filters">
        <label>
          Server
          <select
            value={server}
            onChange={(e) => {
              setServer(e.target.value);
              setLibrary("");
              setTrail([]);
              setType("");
              setOffset(0);
            }}
          >
            <option value="">All servers</option>
            {servers.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </label>
        <label>
          Library
          <select
            value={library}
            onChange={(e) => {
              setLibrary(e.target.value);
              setTrail([]);
              setType("");
              setOffset(0);
            }}
          >
            <option value="">All libraries</option>
            {libraries
              .filter((l) => !server || l.serverId === server)
              .map((l) => (
                <option key={l.id} value={l.id}>
                  {l.title}
                </option>
              ))}
          </select>
        </label>
        <label>
          Type
          <select
            value={type}
            onChange={(e) => {
              setType(e.target.value);
              setTrail([]);
              setOffset(0);
            }}
          >
            <option value="">Movies & shows</option>
            {["movie", "show", "season", "episode"].map((t) => (
              <option key={t}>{t}</option>
            ))}
          </select>
        </label>
        <label>
          Availability
          <select
            value={available}
            onChange={(e) => {
              setAvailable(e.target.value);
              setOffset(0);
            }}
          >
            <option value="">All items</option>
            <option value="true">Available</option>
            <option value="false">Unavailable</option>
          </select>
        </label>
        <label className="search-label">
          Search titles
          <input
            value={title}
            placeholder="Search this view"
            onChange={(e) => {
              setTitle(e.target.value);
              setOffset(0);
            }}
          />
        </label>
      </div>
      {trail.length > 0 && (
        <nav className="card-actions" aria-label="TV library hierarchy">
          <button
            className="secondary"
            onClick={() => {
              setTrail([]);
              setType("");
              setTitle("");
              setOffset(0);
              setDetail(null);
              setServer(rootFilters.server);
              setLibrary(rootFilters.library);
              focusOnLoad.current = true;
            }}
          >
            <ArrowLeft size={16} /> Movies & shows
          </button>
          {trail.map((item, i) => (
            <button
              key={item.id}
              onClick={() => {
                setTrail(trail.slice(0, i + 1));
                focusOnLoad.current = true;
                setType(item.type === "show" ? "season" : "episode");
                setTitle("");
                setOffset(0);
                setDetail(null);
              }}
            >
              {item.title}
            </button>
          ))}
          <button onClick={() => void inspect(trail.at(-1)!.id)}>
            View {trail.at(-1)!.type} details
          </button>
        </nav>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {detail && (
        <section className="media-detail" aria-label="Media details">
          <div className="plex-toolbar">
            <h2>{detail.title}</h2>
            <button
              aria-label="Close media details"
              onClick={() => setDetail(null)}
            >
              <X size={17} />
            </button>
          </div>
          <div className="detail-art">
            <img
              src={`/api/v1/media/${detail.id}/artwork/background`}
              alt="Background artwork"
              onError={(e) => {
                e.currentTarget.hidden = true;
              }}
            />
          </div>
          <p>{detail.summary || "No description supplied by Plex."}</p>
          <p>
            {detail.type} · {detail.year ?? "Year unknown"} ·{" "}
            {detail.contentRating || "Unrated"} · Plex {detail.ratingKey}
          </p>
          <p>
            {detail.tags
              ?.map((t) => `${t.relationship}: ${t.name}`)
              .join(" · ")}
          </p>
          <p>
            {detail.externalIds
              ?.map((x) => `${x.provider}: ${x.value}`)
              .join(" · ")}
          </p>
          {detail.versions?.map((v, i) => (
            <p key={i}>
              {v.resolution} · {v.container} · {v.videoCodec} / {v.audioCodec} ·{" "}
              {v.streams?.length ?? 0} streams
            </p>
          ))}
          {["show", "season"].includes(detail.type) &&
            detail.ratingKey !== parent && (
              <button className="primary" onClick={() => drill(detail)}>
                {detail.type === "show" ? "Browse seasons" : "Browse episodes"}
              </button>
            )}
        </section>
      )}
      {!data ? (
        <p role="status">Loading inventory…</p>
      ) : data.total === 0 ? (
        <div className="empty-panel">
          <Search size={32} />
          <h2>No matching media</h2>
          <p>Sync a selected library, or adjust the filters.</p>
        </div>
      ) : (
        <>
          <p className="plex-help">
            {data.total.toLocaleString()} matching items
          </p>
          <div className="media-grid" ref={gridRef}>
            {data.items.map((item) => (
              <button
                key={item.id}
                className="media-card"
                onClick={() =>
                  ["show", "season"].includes(item.type)
                    ? drill(item)
                    : void inspect(item.id)
                }
              >
                <span className="poster">
                  <Film size={28} />
                  <img
                    loading="lazy"
                    src={`/api/v1/media/${item.id}/artwork/poster`}
                    alt=""
                    onError={(e) => {
                      e.currentTarget.hidden = true;
                    }}
                  />
                </span>
                <strong>{item.title}</strong>
                <span>
                  {item.year ?? "—"} · {item.type}
                  {item.type === "episode" && item.index !== null
                    ? ` ${item.parentIndex !== null ? `S${item.parentIndex} ` : ""}E${item.index}`
                    : ""}
                </span>
                <span>
                  {item.libraryTitle} · Plex {item.ratingKey}
                </span>
                <span className={item.available ? "available" : "missing"}>
                  {item.available ? "Available" : "Unavailable"}
                </span>
                <small>Updated {displayTime(item.updatedAt)}</small>
              </button>
            ))}
          </div>
          <div className="pagination">
            <button
              disabled={offset === 0}
              onClick={() => setOffset(Math.max(0, offset - 24))}
            >
              Previous
            </button>
            <span>
              {offset + 1}–{Math.min(offset + 24, data.total)} of {data.total}
            </span>
            <button
              disabled={offset + 24 >= data.total}
              onClick={() => setOffset(offset + 24)}
            >
              Next
            </button>
          </div>
        </>
      )}
    </>
  );
}
function SyncSettings({ csrf }: { csrf: string }) {
  const [value, setValue] = useState<PlexOptions | null>(null);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    plexAPI
      .settings()
      .then(setValue)
      .catch((e) => setError(errorMessage(e)));
  }, []);
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!value) return;
    setBusy(true);
    setError("");
    setSaved(false);
    try {
      setValue(await plexAPI.saveSettings(value, csrf));
      setSaved(true);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <form className="settings-panel" onSubmit={save}>
      <h2>Keep your library in sync</h2>
      <p>
        Incremental scans check the complete inventory and fetch changed
        metadata. Full refreshes also re-read unchanged metadata. One job runs
        at a time.
      </p>
      {value && (
        <div className="connection-fields">
          {(
            [
              {
                key: "healthIntervalSeconds",
                label: "Health interval (seconds)",
                min: 30,
                max: 86400,
              },
              {
                key: "syncIntervalSeconds",
                label: "Sync interval (seconds)",
                min: 300,
                max: 604800,
              },
              {
                key: "fullSyncIntervalSeconds",
                label: "Full refresh interval (seconds)",
                min: 300,
                max: 2592000,
              },
              {
                key: "artworkCacheMb",
                label: "Artwork cache limit (MB)",
                min: 16,
                max: 10240,
              },
            ] as const
          ).map((field) => (
            <label key={field.key}>
              {field.label}
              <input
                type="number"
                required
                min={field.min}
                max={field.max}
                value={value[field.key]}
                onChange={(e) =>
                  setValue({ ...value, [field.key]: Number(e.target.value) })
                }
              />
            </label>
          ))}
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {saved && (
        <p className="success" role="status">
          Sync settings saved.
        </p>
      )}
      <button className="primary" disabled={busy || !value}>
        Save sync settings
      </button>
    </form>
  );
}
