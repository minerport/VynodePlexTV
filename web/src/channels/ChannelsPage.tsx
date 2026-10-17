import { useCallback, useEffect, useRef, useState } from "react";
import type { Session } from "../api";
import {
  channelAPI,
  definitionOf,
  errorText,
  newDefinition,
  type Channel,
  type Definition,
  type Field,
  type Group,
} from "./api";
import { ChannelEditor } from "./ChannelEditor";
import { GroupManager } from "./GroupManager";
import { Preview } from "./Preview";
import "./channels.css";
export function ChannelsPage({ session }: { session: Session }) {
  if (session.user.role === "USER")
    return (
      <section className="destination">
        <h1>Your channel collection</h1>
        <p>
          An administrator manages your channels. Live programming is coming in
          a later phase.
        </p>
      </section>
    );
  return <ChannelAdmin csrf={session.csrfToken} />;
}
function ChannelAdmin({ csrf }: { csrf: string }) {
  const createButton = useRef<HTMLButtonElement>(null);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [fields, setFields] = useState<Field[]>([]);
  const [offset, setOffset] = useState(0);
  const [more, setMore] = useState(false);
  const [next, setNext] = useState("101");
  const [editor, setEditor] = useState<{
    id?: string;
    draft: Definition;
  } | null>(null);
  const [manage, setManage] = useState(false);
  const [preview, setPreview] = useState<Channel | null>(null);
  const [deleting, setDeleting] = useState<Channel | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [filter, setFilter] = useState("");
  useEffect(() => {
    if (!editor) {
      const frame = requestAnimationFrame(() => createButton.current?.focus());
      return () => cancelAnimationFrame(frame);
    }
  }, [editor]);
  const reload = useCallback(async () => {
    const [list, g, f] = await Promise.all([
      channelAPI.list(offset, filter),
      channelAPI.groups(),
      channelAPI.fields(),
    ]);
    setChannels(list.items);
    setMore(list.more);
    setNext(list.nextNumber);
    setGroups(g);
    setFields(f);
  }, [offset, filter]);
  useEffect(() => {
    const timer = setTimeout(
      () => void reload().catch((e) => setError(errorText(e))),
      0,
    );
    return () => clearTimeout(timer);
  }, [reload]);
  async function run(fn: () => Promise<unknown>, message: string) {
    setBusy(true);
    setError("");
    try {
      await fn();
      await reload();
      setNotice(message);
      setDeleting(null);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  if (editor)
    return (
      <ChannelEditor
        key={editor.id ?? "new"}
        initial={editor.draft}
        id={editor.id}
        groups={groups}
        fields={fields}
        csrf={csrf}
        onClose={() => setEditor(null)}
        onSaved={() => {
          setEditor(null);
          setNotice("Channel saved.");
          void reload().catch((e) => setError(errorText(e)));
        }}
      />
    );
  return (
    <section className="channel-page">
      <div className="destination-heading">
        <span className="eyebrow">YOUR CHANNEL COLLECTION</span>
        <h1>A frequency for every mood.</h1>
        <p>
          Handpick favorites, dedicate a channel to a show, or let smart rules
          find the right mix.
        </p>
      </div>
      <div className="channel-actions">
        <button
          className="primary"
          ref={createButton}
          onClick={() => setEditor({ draft: newDefinition(next) })}
        >
          + Create channel
        </button>
        <button onClick={() => setManage(!manage)}>Manage groups</button>
        <label>
          Group
          <select
            value={filter}
            onChange={(e) => {
              setFilter(e.target.value);
              setOffset(0);
            }}
          >
            <option value="">All groups</option>
            <option value="ungrouped">Ungrouped</option>
            {groups.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name}
              </option>
            ))}
          </select>
        </label>
      </div>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="success" role="status">
          {notice}
        </p>
      )}
      {manage && (
        <GroupManager
          groups={groups}
          csrf={csrf}
          onChange={reload}
          onClose={() => setManage(false)}
        />
      )}
      {deleting && (
        <div
          className="channel-panel"
          role="alertdialog"
          aria-label="Delete channel"
        >
          <h2>
            Delete {deleting.number} · {deleting.name}?
          </h2>
          <p>Your Plex media and other channels will remain available.</p>
          <button
            disabled={busy}
            onClick={() =>
              void run(
                () => channelAPI.remove(deleting.id, csrf),
                "Channel deleted.",
              )
            }
          >
            Confirm delete channel
          </button>
          <button onClick={() => setDeleting(null)}>Cancel</button>
        </div>
      )}
      {preview && (
        <section className="channel-panel">
          <div className="channel-actions">
            <h2>
              {preview.number} · {preview.name}
            </h2>
            <button onClick={() => setPreview(null)}>Close preview</button>
          </div>
          <Preview source={preview.source} csrf={csrf} />
        </section>
      )}
      {channels.length === 0 && !error && (
        <div className="empty-panel">
          <h2>Your first channel starts here.</h2>
          <p>
            Use your synchronized Plex library to give it a personality.
            Scheduling and playback come next.
          </p>
        </div>
      )}
      <div className="channel-grid">
        {channels
          .filter(
            (c) =>
              !filter ||
              (filter === "ungrouped" ? !c.groupId : c.groupId === filter),
          )
          .map((c) => (
            <article className="channel-card" key={c.id}>
              <div
                className="channel-card-art"
                style={{
                  backgroundImage: `linear-gradient(0deg,#17151e,transparent),url(/api/v1/channels/${c.id}/artwork/background?v=${c.updatedAt})`,
                }}
              >
                <img
                  loading="lazy"
                  src={`/api/v1/channels/${c.id}/artwork/logo?v=${c.updatedAt}`}
                  alt=""
                />
                <span className="channel-number">{c.number}</span>
              </div>
              <div className="channel-card-body">
                <span className="eyebrow">
                  {c.source.type} · {c.enabled ? "ENABLED" : "DISABLED"}
                  {c.visibility === "HIDDEN" ? " · HIDDEN" : ""}
                </span>
                <h2>{c.name}</h2>
                <p>
                  {c.description ||
                    groups.find((g) => g.id === c.groupId)?.name ||
                    "Your own frequency"}
                </p>
                <strong>{c.count.toLocaleString()} playable items</strong>
                <p>
                  <small>
                    {groups.find((g) => g.id === c.groupId)?.name ??
                      "Ungrouped"}
                  </small>
                </p>
                <p>
                  <small>
                    Updated {new Date(c.updatedAt * 1000).toLocaleString()}
                  </small>
                </p>
                {c.warnings?.map((w) => (
                  <p className="notice" key={w}>
                    {w}
                  </p>
                ))}
                <div className="channel-actions">
                  <button
                    onClick={() =>
                      setEditor({ id: c.id, draft: definitionOf(c) })
                    }
                  >
                    Edit
                  </button>
                  <button onClick={() => setPreview(c)}>Preview</button>
                  <button
                    disabled={busy}
                    onClick={() =>
                      void run(
                        () => channelAPI.duplicate(c.id, csrf),
                        "Channel duplicated with a new number.",
                      )
                    }
                  >
                    Duplicate
                  </button>
                  <button
                    disabled={busy}
                    onClick={() =>
                      void run(
                        () => channelAPI.enable(c.id, !c.enabled, csrf),
                        c.enabled ? "Channel disabled." : "Channel enabled.",
                      )
                    }
                  >
                    {c.enabled ? "Disable" : "Enable"}
                  </button>
                  <button onClick={() => setDeleting(c)}>Delete</button>
                </div>
              </div>
            </article>
          ))}
      </div>
      {(offset > 0 || more) && (
        <div className="channel-actions">
          <button
            disabled={offset === 0}
            onClick={() => setOffset(Math.max(0, offset - 24))}
          >
            Previous channels
          </button>
          <span>Page {Math.floor(offset / 24) + 1}</span>
          <button disabled={!more} onClick={() => setOffset(offset + 24)}>
            More channels
          </button>
        </div>
      )}
    </section>
  );
}
