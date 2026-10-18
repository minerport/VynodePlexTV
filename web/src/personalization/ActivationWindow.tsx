import { useEffect, useState } from "react";
import { request } from "../api";
export function ActivationWindow({ id, csrf }: { id: string; csrf: string }) {
  const [v, setV] = useState({
      enabled: false,
      startDate: "",
      endDate: "",
      timezone: "UTC",
    }),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    void request<typeof v>(`/channels/${id}/activation`)
      .then(setV)
      .catch(() => setMessage("Activation settings unavailable"));
  }, [id]);
  async function save(remove: boolean) {
    setBusy(true);
    try {
      await request(
        `/channels/${id}/activation`,
        remove ? "DELETE" : "PATCH",
        remove
          ? {}
          : {
              startDate: v.startDate,
              endDate: v.endDate,
              timezone: v.timezone,
            },
        csrf,
      );
      setV({ ...v, enabled: !remove });
      setMessage(
        "Saved. Future repair queued; current programming is protected.",
      );
    } catch (e) {
      setMessage(e instanceof Error ? e.message : "Unable to save activation");
    } finally {
      setBusy(false);
    }
  }
  return (
    <details>
      <summary>
        Seasonal activation {v.enabled ? "· Enabled" : "· Always active"}
      </summary>
      <p>
        Inclusive local dates, with explicit years. Active programs finish
        normally.
      </p>
      <label>
        Start date
        <input
          type="date"
          value={v.startDate}
          onChange={(e) => setV({ ...v, startDate: e.target.value })}
        />
      </label>
      <label>
        End date
        <input
          type="date"
          value={v.endDate}
          onChange={(e) => setV({ ...v, endDate: e.target.value })}
        />
      </label>
      <label>
        Activation timezone
        <input
          value={v.timezone}
          onChange={(e) => setV({ ...v, timezone: e.target.value })}
        />
      </label>
      <button
        type="button"
        disabled={busy || !v.startDate || !v.endDate}
        onClick={() => void save(false)}
      >
        Save activation window
      </button>
      {v.enabled && (
        <button type="button" disabled={busy} onClick={() => void save(true)}>
          Make always active
        </button>
      )}
      {message && <p role="status">{message}</p>}
    </details>
  );
}
