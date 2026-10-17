import { useState, type FormEvent } from "react";
import { channelAPI, errorText, type Group } from "./api";
const empty = { name: "", slug: "", description: "", sortOrder: 0 };
export function GroupManager({
  groups,
  csrf,
  onChange,
  onClose,
}: {
  groups: Group[];
  csrf: string;
  onChange: () => Promise<void>;
  onClose: () => void;
}) {
  const [draft, setDraft] = useState<Partial<Group>>(empty);
  const [deleting, setDeleting] = useState<Group | null>(null);
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function run(fn: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await fn();
      await onChange();
      setDraft(empty);
      setDeleting(null);
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  function save(e: FormEvent) {
    e.preventDefault();
    void run(() => channelAPI.saveGroup(draft, csrf));
  }
  return (
    <section className="channel-panel">
      <div className="channel-actions">
        <h2>Channel groups</h2>
        <button onClick={onClose}>Close groups</button>
      </div>
      <p>Organize channels. Deleting a group never deletes its channels.</p>
      {groups.map((g, i) => (
        <div key={g.id} className="selected-row">
          <span>
            <strong>{g.name}</strong> · {g.count} channels
          </span>
          <button
            disabled={busy || i === 0}
            aria-label={`Move ${g.name} up`}
            onClick={() =>
              void run(async () => {
                const reordered = [...groups];
                [reordered[i - 1], reordered[i]] = [
                  reordered[i],
                  reordered[i - 1],
                ];
                for (let n = 0; n < reordered.length; n++) {
                  const x = reordered[n];
                  await channelAPI.saveGroup(
                    {
                      id: x.id,
                      name: x.name,
                      slug: x.slug,
                      description: x.description,
                      sortOrder: n,
                    },
                    csrf,
                  );
                }
              })
            }
          >
            ↑
          </button>
          <button
            onClick={() =>
              setDraft({
                id: g.id,
                name: g.name,
                slug: g.slug,
                description: g.description,
                sortOrder: g.sortOrder,
              })
            }
          >
            Edit
          </button>
          <button
            onClick={() => {
              setDeleting(g);
              setTarget("");
            }}
          >
            Delete
          </button>
        </div>
      ))}
      {deleting && (
        <div className="notice" role="alertdialog" aria-label="Delete group">
          <h3>Delete {deleting.name}?</h3>
          <label>
            Keep its channels in
            <select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Ungrouped</option>
              {groups
                .filter((g) => g.id !== deleting.id)
                .map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name}
                  </option>
                ))}
            </select>
          </label>
          <button
            disabled={busy}
            onClick={() =>
              void run(() =>
                channelAPI.removeGroup(
                  deleting.id,
                  target || null,
                  !target,
                  csrf,
                ),
              )
            }
          >
            Confirm delete group
          </button>
          <button onClick={() => setDeleting(null)}>Cancel</button>
        </div>
      )}
      <form onSubmit={save}>
        <h3>{draft.id ? "Edit group" : "Create group"}</h3>
        <div className="channel-filters">
          <label>
            Group name
            <input
              required
              value={draft.name}
              onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            />
          </label>
          <label>
            Slug
            <input
              required
              pattern="[a-z0-9-]+"
              value={draft.slug}
              onChange={(e) => setDraft({ ...draft, slug: e.target.value })}
            />
          </label>
          <label>
            Order
            <input
              type="number"
              value={draft.sortOrder}
              onChange={(e) =>
                setDraft({ ...draft, sortOrder: Number(e.target.value) })
              }
            />
          </label>
        </div>
        <label>
          Group description
          <input
            value={draft.description}
            onChange={(e) =>
              setDraft({ ...draft, description: e.target.value })
            }
          />
        </label>
        <button className="primary" disabled={busy}>
          Save group
        </button>
        {draft.id && (
          <button type="button" onClick={() => setDraft(empty)}>
            New group
          </button>
        )}
      </form>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
