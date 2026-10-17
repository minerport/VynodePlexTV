import { useEffect, useRef, useState, type FormEvent } from "react";
import { APIError } from "../api";
import {
  channelAPI,
  errorText,
  newSource,
  type Channel,
  type Definition,
  type Field,
  type Group,
  type Source,
} from "./api";
import { MediaPicker } from "./MediaPicker";
import { MetadataPicker } from "./MetadataPicker";
import { RuleBuilder } from "./RuleBuilder";
import { Preview } from "./Preview";
import { ScheduleAdmin } from "../scheduler/ScheduleAdmin";
export function ChannelEditor({
  initial,
  id,
  groups,
  fields,
  csrf,
  onSaved,
  onClose,
}: {
  initial: Definition;
  id?: string;
  groups: Group[];
  fields: Field[];
  csrf: string;
  onSaved: (c: Channel) => void;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState(initial);
  const [scheduling, setScheduling] = useState(false);
  const nameInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    nameInput.current?.focus();
  }, [scheduling]);
  useEffect(() => {
    const back = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !scheduling) {
        e.preventDefault();
        e.stopImmediatePropagation();
        onClose();
      }
    };
    window.addEventListener("keydown", back, true);
    return () => window.removeEventListener("keydown", back, true);
  }, [onClose, scheduling]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [issues, setIssues] = useState<{ path: string; message: string }[]>([]);
  const [artPicker, setArtPicker] = useState(false);
  const patch = (p: Partial<Definition>) => setDraft((d) => ({ ...d, ...p }));
  const source = (s: Source) => patch({ source: s });
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setIssues([]);
    try {
      onSaved(await channelAPI.save(id, draft, csrf));
    } catch (e) {
      setError(errorText(e));
      if (e instanceof APIError) setIssues(e.issues);
    } finally {
      setBusy(false);
    }
  }
  async function upload(kind: "logo" | "background", file?: File) {
    if (!file) return;
    setBusy(true);
    setError("");
    try {
      const u = await channelAPI.upload(file, csrf);
      patch({ [kind]: { source: "UPLOAD", uploadId: u.id } });
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  if (scheduling && id)
    return (
      <ScheduleAdmin
        csrf={csrf}
        initialId={id}
        onClose={() => setScheduling(false)}
      />
    );
  return (
    <div className="channel-editor">
      <div className="channel-actions">
        <button onClick={onClose}>← All channels</button>
        {id && (
          <button onClick={() => setScheduling(true)}>
            Scheduling settings & preview
          </button>
        )}
        <span className="eyebrow">
          {id ? "REFINE YOUR CHANNEL" : "CREATE A FREQUENCY"}
        </span>
      </div>
      <h1>{id ? "Edit channel" : "Make it yours."}</h1>
      <form onSubmit={save}>
        <div className="channel-editor-layout">
          <div className="channel-editor-fields">
            <section className="channel-panel">
              <h2>Channel identity</h2>
              <div className="channel-filters">
                <label>
                  Channel number
                  <input
                    required
                    maxLength={24}
                    value={draft.number}
                    onChange={(e) => patch({ number: e.target.value })}
                  />
                </label>
                <label>
                  Channel name
                  <input
                    required
                    maxLength={100}
                    ref={nameInput}
                    value={draft.name}
                    onChange={(e) => patch({ name: e.target.value })}
                  />
                </label>
                <label>
                  Short name
                  <input
                    maxLength={24}
                    value={draft.shortName}
                    onChange={(e) => patch({ shortName: e.target.value })}
                  />
                </label>
                <label>
                  Group
                  <select
                    value={draft.groupId ?? ""}
                    onChange={(e) => patch({ groupId: e.target.value || null })}
                  >
                    <option value="">Ungrouped</option>
                    {groups.map((g) => (
                      <option key={g.id} value={g.id}>
                        {g.name}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <label>
                Description
                <textarea
                  maxLength={2000}
                  value={draft.description}
                  onChange={(e) => patch({ description: e.target.value })}
                />
              </label>
              <div className="channel-filters">
                <label>
                  Visibility
                  <select
                    value={draft.visibility}
                    onChange={(e) =>
                      patch({
                        visibility: e.target.value as Definition["visibility"],
                      })
                    }
                  >
                    <option value="VISIBLE">Visible</option>
                    <option value="HIDDEN">Hidden</option>
                  </select>
                </label>
                <label>
                  Display order
                  <input
                    type="number"
                    value={draft.sortOrder}
                    onChange={(e) =>
                      patch({ sortOrder: Number(e.target.value) })
                    }
                  />
                </label>
                <label className="check-label">
                  <input
                    type="checkbox"
                    checked={draft.enabled}
                    onChange={(e) => patch({ enabled: e.target.checked })}
                  />
                  Enabled
                </label>
              </div>
            </section>
            <section className="channel-panel">
              <h2>Content source</h2>
              <label>
                Channel type
                <select
                  value={draft.source.type}
                  onChange={(e) =>
                    source(newSource(e.target.value as Source["type"]))
                  }
                >
                  <option value="STATIC">Handpicked movies & episodes</option>
                  <option value="SHOW">Dedicated TV show</option>
                  <option value="COLLECTION">Plex collection</option>
                  <option value="RULE">Smart rules</option>
                </select>
              </label>
              {draft.source.static && (
                <MediaPicker
                  ids={draft.source.static.mediaIds}
                  csrf={csrf}
                  onChange={(mediaIds) =>
                    source({ type: "STATIC", static: { mediaIds } })
                  }
                />
              )}
              {draft.source.show && (
                <>
                  <MetadataPicker
                    field="show"
                    label="TV show"
                    values={
                      draft.source.show.showId ? [draft.source.show.showId] : []
                    }
                    onChange={(v) =>
                      source({
                        ...draft.source,
                        show: {
                          ...draft.source.show!,
                          showId: v[0] ?? "",
                          seasonIds: [],
                        },
                      })
                    }
                  />
                  <label>
                    Episode selection
                    <select
                      value={draft.source.show.episodeScope}
                      onChange={(e) =>
                        source({
                          ...draft.source,
                          show: {
                            ...draft.source.show!,
                            episodeScope: e.target.value as
                              "ALL" | "SELECTED_SEASONS",
                          },
                        })
                      }
                    >
                      <option value="ALL">All seasons</option>
                      <option value="SELECTED_SEASONS">Selected seasons</option>
                    </select>
                  </label>
                  {draft.source.show.episodeScope === "SELECTED_SEASONS" &&
                    draft.source.show.showId && (
                      <MetadataPicker
                        field="season"
                        label="Seasons"
                        parent={draft.source.show.showId}
                        values={draft.source.show.seasonIds}
                        multiple
                        onChange={(v) =>
                          source({
                            ...draft.source,
                            show: { ...draft.source.show!, seasonIds: v },
                          })
                        }
                      />
                    )}
                  <label className="check-label">
                    <input
                      type="checkbox"
                      checked={draft.source.show.includeSpecials}
                      onChange={(e) =>
                        source({
                          ...draft.source,
                          show: {
                            ...draft.source.show!,
                            includeSpecials: e.target.checked,
                          },
                        })
                      }
                    />
                    Include specials (season 0)
                  </label>
                  <label className="check-label">
                    <input
                      type="checkbox"
                      checked={draft.source.show.excludeUnavailable}
                      onChange={(e) =>
                        source({
                          ...draft.source,
                          show: {
                            ...draft.source.show!,
                            excludeUnavailable: e.target.checked,
                          },
                        })
                      }
                    />
                    Exclude unavailable episodes
                  </label>
                  <p className="field-help">
                    Episodes resolve in season and episode order. Choose at
                    least one season when using selected seasons.
                  </p>
                </>
              )}
              {draft.source.collection && (
                <MetadataPicker
                  field="collection"
                  label="Plex collection"
                  values={
                    draft.source.collection.collectionId
                      ? [draft.source.collection.collectionId]
                      : []
                  }
                  onChange={(v) =>
                    source({
                      type: "COLLECTION",
                      collection: { collectionId: v[0] ?? "" },
                    })
                  }
                />
              )}
              {draft.source.rule && (
                <RuleBuilder
                  rule={draft.source.rule}
                  fields={fields}
                  issues={issues}
                  onChange={(rule) => source({ type: "RULE", rule })}
                />
              )}
            </section>
            <section className="channel-panel">
              <h2>Artwork & atmosphere</h2>
              {(["logo", "background"] as const).map((kind) => (
                <div key={kind}>
                  <label>
                    {kind === "logo" ? "Channel logo" : "Background"}
                    <select
                      value={draft[kind].source}
                      onChange={(e) =>
                        patch({
                          [kind]: { ...draft[kind], source: e.target.value },
                        })
                      }
                    >
                      <option value="AUTO">Automatic from content</option>
                      <option value="PLEX">Selected Plex artwork</option>
                      <option value="UPLOAD">Custom upload</option>
                    </select>
                  </label>
                  {draft[kind].source === "UPLOAD" && (
                    <>
                      <label>
                        Upload {kind} · PNG, JPEG or WebP (8 MB maximum)
                        <input
                          type="file"
                          accept="image/png,image/jpeg,image/webp"
                          disabled={busy}
                          onChange={(e) =>
                            void upload(kind, e.target.files?.[0])
                          }
                        />
                      </label>
                      {draft[kind].uploadId && (
                        <img
                          className="channel-upload-preview"
                          src={`/api/v1/channel-artwork/${draft[kind].uploadId}`}
                          alt={`Uploaded ${kind}`}
                        />
                      )}
                    </>
                  )}
                </div>
              ))}
              {(draft.logo.source === "PLEX" ||
                draft.background.source === "PLEX") && (
                <>
                  <button
                    type="button"
                    onClick={() => setArtPicker(!artPicker)}
                  >
                    {artPicker
                      ? "Close artwork picker"
                      : "Choose artwork from media"}
                  </button>
                  {draft.artworkMediaId && (
                    <img
                      className="channel-upload-preview"
                      src={`/api/v1/media/${draft.artworkMediaId}/artwork/poster`}
                      alt="Selected Plex artwork"
                    />
                  )}
                  {artPicker && (
                    <MediaPicker
                      single
                      ids={draft.artworkMediaId ? [draft.artworkMediaId] : []}
                      csrf={csrf}
                      onChange={(ids) =>
                        patch({ artworkMediaId: ids[0] ?? null })
                      }
                    />
                  )}
                </>
              )}
            </section>
          </div>
          <aside>
            <div className="channel-identity-preview">
              <span>{draft.number || "—"}</span>
              <h2>{draft.name || "Your channel"}</h2>
              <p>
                {draft.description ||
                  "Your library, with a personality of its own."}
              </p>
            </div>
            <Preview source={draft.source} csrf={csrf} />
          </aside>
        </div>
        {error && (
          <div className="error" role="alert">
            <p>{error}</p>
            {issues.map((i, n) => (
              <p key={n}>
                {i.path}: {i.message}
              </p>
            ))}
          </div>
        )}
        <div className="channel-save">
          <button className="primary" disabled={busy}>
            {busy ? "Saving…" : "Save channel"}
          </button>
          <button type="button" onClick={onClose}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
