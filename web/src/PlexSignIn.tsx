import { useEffect, useState } from "react";
import { ExternalLink, ShieldCheck } from "lucide-react";
import { APIError, request } from "./api";
import type { PlexServer } from "./plex-api";

interface Link {
  id: string;
  authUrl: string;
  expiresAt: number;
}
interface Resource {
  id: string;
  name: string;
  owned: boolean;
  connections: {
    uri: string;
    local: boolean;
    relay: boolean;
    protocol: string;
  }[];
}
interface Status {
  status: "WAITING" | "AUTHORIZED";
  servers: Resource[];
}
export function PlexSignIn({
  csrf,
  onConnected,
}: {
  csrf: string;
  onConnected: (server: PlexServer) => Promise<void>;
}) {
  const [link, setLink] = useState<Link | null>(null);
  const [status, setStatus] = useState<Status | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [choices, setChoices] = useState<Record<string, string>>({});
  async function start() {
    setBusy(true);
    setError("");
    setStatus(null);
    setLink(null);
    setChoices({});
    try {
      setLink(await request<Link>("/plex/link", "POST", undefined, csrf));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to start Plex sign-in");
    } finally {
      setBusy(false);
    }
  }
  useEffect(() => {
    if (!link || status?.status === "AUTHORIZED") return;
    let active = true;
    let inFlight = false;
    const timer = window.setInterval(() => {
      if (inFlight) return;
      inFlight = true;
      request<Status>(`/plex/link/${link.id}/poll`, "POST", undefined, csrf)
        .then((value) => {
          if (active) {
            setStatus(value);
            setError("");
          }
        })
        .catch((e) => {
          if (active) {
            setError(e instanceof Error ? e.message : "Plex sign-in failed");
            if (Date.now() / 1000 >= link.expiresAt)
              window.clearInterval(timer);
          }
        })
        .finally(() => {
          inFlight = false;
        });
    }, 2500);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [link, status?.status, csrf]);
  async function cancel() {
    if (link) {
      try {
        await request(`/plex/link/${link.id}`, "DELETE", undefined, csrf);
      } catch (e) {
        if (!(e instanceof APIError && e.status === 404)) {
          setError(
            e instanceof Error ? e.message : "Unable to cancel Plex sign-in",
          );
          return;
        }
      }
    }
    setLink(null);
    setStatus(null);
    setError("");
  }
  async function connect(resource: Resource) {
    if (!link) return;
    setBusy(true);
    setError("");
    try {
      const server = await request<PlexServer>(
        `/plex/link/${link.id}/connect`,
        "POST",
        {
          serverId: resource.id,
          uri: choices[resource.id] ?? "",
        },
        csrf,
      );
      setLink(null);
      setStatus(null);
      await onConnected(server);
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : "Could not connect to this Plex server",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="plex-signin">
      <div>
        <ShieldCheck size={26} />
        <h2>Connect with your Plex account</h2>
        <p>
          Sign in securely on Plex, then choose your server. Your Plex password
          is never entered here.
        </p>
      </div>
      {!link ? (
        <button
          className="primary"
          disabled={busy}
          onClick={() => void start()}
        >
          {busy ? "Preparing sign-in…" : "Sign in with Plex"}
          <ExternalLink size={17} />
        </button>
      ) : status?.status !== "AUTHORIZED" ? (
        <div className="signin-wait">
          <a
            className="primary signin-link"
            href={link.authUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            Open Plex sign-in <ExternalLink size={17} />
          </a>
          <p role="status">
            Complete sign-in in the Plex tab, then return here. Waiting for
            authorization…
          </p>
          <button className="secondary" onClick={() => void cancel()}>
            Cancel sign-in
          </button>
        </div>
      ) : (
        <div className="linked-servers">
          <h3>Choose your Plex server</h3>
          {status.servers.length === 0 ? (
            <p>
              No servers with library access were returned by Plex. Check the
              account you signed into.
            </p>
          ) : (
            status.servers.map((resource) => (
              <div key={resource.id} className="linked-server">
                <strong>
                  {resource.name}{" "}
                  <small>{resource.owned ? "Owned" : "Shared"}</small>
                </strong>
                <label>
                  Connection for {resource.name}
                  <select
                    value={choices[resource.id] ?? ""}
                    onChange={(e) =>
                      setChoices({ ...choices, [resource.id]: e.target.value })
                    }
                  >
                    <option value="">Automatic connection (recommended)</option>
                    {resource.connections.map((c) => (
                      <option key={c.uri} value={c.uri}>
                        {c.local ? "Local" : "Remote"}
                        {c.relay ? " relay" : ""}
                        {c.protocol === "http" ? " · Unencrypted" : ""} ·{" "}
                        {c.uri}
                      </option>
                    ))}
                  </select>
                </label>
                <button
                  disabled={busy || resource.connections.length === 0}
                  className="primary"
                  onClick={() => void connect(resource)}
                >
                  {busy ? "Connecting…" : "Connect this server"}
                </button>
              </div>
            ))
          )}
          <button
            className="secondary"
            disabled={busy}
            onClick={() => void cancel()}
          >
            Cancel sign-in
          </button>
        </div>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
