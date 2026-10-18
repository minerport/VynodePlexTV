import { useEffect, useState } from "react";
import { channelAPI } from "../channels/api";
import { request, type Session } from "../api";
import {
  presentationAPI as api,
  upload,
  kinds,
  type Asset,
  type Rule,
  type Branding,
} from "./api";
import { BroadcastOverlay, type BroadcastContext } from "./BroadcastOverlay";

import "./presentation.css";
const newRule: Rule = {
  id: "",
  name: "",
  channelId: "",
  enabled: true,
  kind: "BUMPER",
  tag: "",
  placement: "BEFORE",
  mediaType: "",
  every: 1,
  repeatMinutes: 120,
  maxDurationMs: 600000,
  priority: 0,
};
export function PresentationAdmin({ session }: { session: Session }) {
  const [assets, setAssets] = useState<Asset[]>([]),
    [rules, setRules] = useState<Rule[]>([]),
    [channels, setChannels] = useState<
      { id: string; name: string; number: string }[]
    >([]),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [rule, setRule] = useState<Rule>(newRule),
    [branding, setBranding] = useState<Branding | null>(null),
    [editing, setEditing] = useState<Asset | null>(null),
    [preview, setPreview] = useState(""),
    [progress, setProgress] = useState<number | null>(null),
    [file, setFile] = useState<File | null>(null),
    [name, setName] = useState(""),
    [kind, setKind] = useState("BUMPER"),
    [mediaId, setMediaId] = useState("");
  const csrf = session.csrfToken;
  async function load() {
    const [a, r] = await Promise.all([api.assets(), api.rules()]);
    setAssets(a);
    setRules(r);
  }
  useEffect(() => {
    void Promise.resolve()
      .then(load)
      .catch((e) => setError(e.message));
    let active = true;
    void (async () => {
      const all: { id: string; name: string; number: string }[] = [];
      let offset = 0;
      for (;;) {
        const page = await request<{ items: typeof all; more: boolean }>(
          `/channels?offset=${offset}&limit=100`,
        );
        all.push(...page.items);
        if (!page.more) break;
        offset += page.items.length;
        if (!page.items.length) throw Error("Channel list did not advance");
      }
      if (active) setChannels(all);
    })().catch((e) => setError(e.message));
    return () => {
      active = false;
    };
  }, []);
  async function act(
    fn: () => Promise<unknown>,
    message = "Saved. Future changes apply through schedule repair.",
  ) {
    setError("");
    try {
      await fn();
      await load();
      setNotice(message);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : "Presentation operation failed",
      );
    }
  }
  if (session.user.role === "USER")
    return <p>Broadcast administration requires an administrator.</p>;
  const channel = channels.find((c) => c.id === branding?.channelId);
  const sample: BroadcastContext = {
    channel: {
      id: channel?.id || "",
      name: channel?.name || "Your network",
      number: channel?.number || "",
      logo: channel ? `/api/v1/watch/channels/${channel.id}/artwork/logo` : "",
      recentKind: "",
    },
    program: { end: 45000 },
    next: {
      title: "Your next program",
      start: 45000,
      showTitle: "",
      season: null,
      episode: null,
    },
    origin: "LIVE",
    mode: "LIVE",
    state: "PLAYING",
    serverTime: 10000,
    delivery: "PREVIEW",
    branding: branding ?? undefined,
  };
  return (
    <div className="presentation-admin">
      <p className="eyebrow">YOUR NETWORK IDENTITY</p>
      <h1>Broadcast Presentation</h1>
      <p>
        Real clips. Real broadcast time. Preview safely, then repair future
        schedules to apply rules.
      </p>
      {error && <p role="alert">{error}</p>}
      {notice && <p role="status">{notice}</p>}
      <section>
        <h2>Asset library</h2>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!file) {
              setError("Choose an MP4 file");
              return;
            }
            if (!file.name.toLowerCase().endsWith(".mp4")) {
              setError("Choose an MP4/H.264/AAC clip");
              return;
            }
            setProgress(0);
            void act(async () => {
              await upload(file, name, kind, csrf, setProgress);
              setFile(null);
            }, "Upload inspected and saved.").finally(() => setProgress(null));
          }}
        >
          <div className="presentation-fields">
            <label>
              Clip name
              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </label>
            <label>
              Asset type
              <select value={kind} onChange={(e) => setKind(e.target.value)}>
                {kinds.map((k) => (
                  <option key={k}>{k}</option>
                ))}
              </select>
            </label>
          </div>
          <div
            className="presentation-drop"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
              e.preventDefault();
              setFile(e.dataTransfer.files[0] || null);
            }}
          >
            <label>
              MP4 video · H.264 / AAC · up to 1080p
              <input
                type="file"
                accept="video/mp4,.mp4"
                onChange={(e) => setFile(e.target.files?.[0] || null)}
              />
            </label>
            {file?.name}
          </div>
          <button disabled={progress !== null}>Upload clip</button>
          {progress !== null && (
            <>
              <progress max={100} value={progress} />
              <span>
                {progress}% {progress === 100 ? "· inspecting video" : ""}
              </span>
            </>
          )}
        </form>
        <button
          onClick={() =>
            void act(() =>
              api.asset(
                {
                  name: "Coming Up Next",
                  kind: "COMING_UP_NEXT",
                  source: "GENERATED_METADATA",
                  durationMs: 10000,
                  enabled: true,
                  tags: [],
                  activeFrom: "",
                  activeUntil: "",
                },
                csrf,
              ),
            )
          }
        >
          Create 10-second Coming Up Next card
        </button>
        <details>
          <summary>Use Plex media / discover trailers</summary>
          <label>
            Local media identity
            <input
              value={mediaId}
              onChange={(e) => setMediaId(e.target.value)}
              placeholder="Select an identity from Plex media administration"
            />
          </label>
          <button
            onClick={() =>
              void act(() =>
                api.asset(
                  {
                    name: name || "Plex presentation",
                    kind,
                    source: "PLEX_MEDIA",
                    mediaId,
                    enabled: true,
                    tags: [],
                    activeFrom: "",
                    activeUntil: "",
                  },
                  csrf,
                ),
              )
            }
          >
            Add Plex clip
          </button>
          <button
            onClick={() =>
              void act(
                () =>
                  request("/interstitials/trailers", "POST", { mediaId }, csrf),
                "Plex trailer discovery completed; unavailable extras are skipped.",
              )
            }
          >
            Discover Plex trailers
          </button>
        </details>
        <div className="presentation-assets">
          {assets.map((a) => (
            <article className="presentation-asset" key={a.id}>
              {a.source === "UPLOAD" ? (
                <video
                  className="asset-thumbnail"
                  aria-label={`${a.name} preview`}
                  preload="metadata"
                  controls={preview === a.id}
                  muted={preview !== a.id}
                  src={`/api/v1/interstitials/assets/${a.id}/preview#t=0.1`}
                />
              ) : a.mediaId ? (
                <img
                  className="asset-thumbnail"
                  loading="lazy"
                  alt=""
                  src={`/api/v1/media/${a.mediaId}/artwork/poster`}
                />
              ) : (
                <div className="asset-thumbnail metadata-thumbnail">
                  COMING UP NEXT
                </div>
              )}
              <h3>{a.name}</h3>
              <p>
                {a.kind} · {Math.round(a.durationMs / 1000)}s · {a.source}
              </p>
              <p>
                {a.tags.join(" · ")} · {a.enabled ? "Enabled" : "Disabled"}{" "}
                {!a.available && "· Unavailable"}
              </p>
              {a.source === "UPLOAD" && (
                <button
                  onClick={() => setPreview(preview === a.id ? "" : a.id)}
                >
                  Preview clip
                </button>
              )}
              <button onClick={() => setEditing(a)}>Edit asset</button>
              <button
                onClick={() =>
                  void act(() => api.asset({ ...a, enabled: !a.enabled }, csrf))
                }
              >
                {a.enabled ? "Disable" : "Enable"}
              </button>
              <button
                onClick={() => {
                  if (
                    confirm(
                      "Delete this asset? Referenced broadcast assets cannot be deleted.",
                    )
                  )
                    void act(() => api.remove("assets", a.id, csrf));
                }}
              >
                Delete
              </button>
            </article>
          ))}
        </div>
        {editing && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void act(async () => {
                await api.asset(editing, csrf);
                setEditing(null);
              });
            }}
          >
            <h3>Edit asset</h3>
            <div className="presentation-fields">
              <label>
                Name
                <input
                  value={editing.name}
                  onChange={(e) =>
                    setEditing({ ...editing, name: e.target.value })
                  }
                />
              </label>
              <label>
                Tags, comma separated
                <input
                  value={editing.tags.join(",")}
                  onChange={(e) =>
                    setEditing({
                      ...editing,
                      tags: e.target.value.split(",").filter(Boolean),
                    })
                  }
                />
              </label>
              <label>
                Active from
                <input
                  type="date"
                  value={editing.activeFrom}
                  onChange={(e) =>
                    setEditing({ ...editing, activeFrom: e.target.value })
                  }
                />
              </label>
              <label>
                Active until
                <input
                  type="date"
                  value={editing.activeUntil}
                  onChange={(e) =>
                    setEditing({ ...editing, activeUntil: e.target.value })
                  }
                />
              </label>
            </div>
            <button>Save asset</button>
            <button type="button" onClick={() => setEditing(null)}>
              Cancel
            </button>
          </form>
        )}
      </section>
      <section>
        <h2>Presentation rules</h2>
        <p>
          Rules are ordered by priority. Hourly means the first safe feature
          boundary after the hour changes.
        </p>
        {rules.map((r) => (
          <div className="rule-row" key={r.id}>
            {r.name} · {r.kind} · every {r.every} ·{" "}
            {r.channelId
              ? channels.find((c) => c.id === r.channelId)?.name
              : "Global default"}{" "}
            · {r.enabled ? "Enabled" : "Disabled"}
            <button onClick={() => setRule(r)}>Edit rule</button>
            <button
              onClick={() =>
                void act(() => api.rule({ ...r, enabled: !r.enabled }, csrf))
              }
            >
              Toggle rule
            </button>
            <button
              onClick={() => {
                if (confirm("Remove rule from future scheduling?"))
                  void act(() => api.remove("rules", r.id, csrf));
              }}
            >
              Delete rule
            </button>
          </div>
        ))}
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void act(async () => {
              await api.rule(rule, csrf);
              setRule(newRule);
            });
          }}
        >
          <div className="presentation-fields">
            <label>
              Rule name
              <input
                required
                value={rule.name}
                onChange={(e) => setRule({ ...rule, name: e.target.value })}
              />
            </label>
            <label>
              Channel
              <select
                value={rule.channelId}
                onChange={(e) =>
                  setRule({ ...rule, channelId: e.target.value })
                }
              >
                <option value="">Global default</option>
                {channels.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.number} · {c.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Asset type
              <select
                value={rule.kind}
                onChange={(e) => setRule({ ...rule, kind: e.target.value })}
              >
                {kinds.map((k) => (
                  <option key={k}>{k}</option>
                ))}
              </select>
            </label>
            <label>
              Placement
              <select
                value={rule.placement}
                onChange={(e) =>
                  setRule({ ...rule, placement: e.target.value })
                }
              >
                <option value="BEFORE">Before program</option>
                <option value="BETWEEN">Between programs</option>
                <option value="HOURLY">First boundary after hour change</option>
              </select>
            </label>
            <label>
              Program type
              <select
                value={rule.mediaType}
                onChange={(e) =>
                  setRule({ ...rule, mediaType: e.target.value })
                }
              >
                <option value="">Any</option>
                <option value="movie">Movies</option>
                <option value="episode">Episodes</option>
              </select>
            </label>
            <label>
              Asset tag
              <input
                value={rule.tag}
                onChange={(e) => setRule({ ...rule, tag: e.target.value })}
              />
            </label>
            {(["every", "repeatMinutes", "priority"] as const).map((key) => (
              <label key={key}>
                {key === "every"
                  ? "Every N programs"
                  : key === "repeatMinutes"
                    ? "Repeat window (minutes)"
                    : "Priority"}
                <input
                  type="number"
                  min={key === "every" ? 1 : 0}
                  value={rule[key]}
                  onChange={(e) => setRule({ ...rule, [key]: +e.target.value })}
                />
              </label>
            ))}
            <label>
              Maximum clip duration (seconds)
              <input
                type="number"
                min="1"
                max="600"
                value={rule.maxDurationMs / 1000}
                onChange={(e) =>
                  setRule({ ...rule, maxDurationMs: +e.target.value * 1000 })
                }
              />
            </label>
          </div>
          <button>Save rule</button>
          <button type="button" onClick={() => setRule(newRule)}>
            New rule
          </button>
        </form>
      </section>
      <section>
        <h2>Channel presentation & preview</h2>
        <label>
          Choose channel
          <select
            value={branding?.channelId || ""}
            onChange={(e) => {
              if (e.target.value)
                void api
                  .branding(e.target.value)
                  .then(setBranding)
                  .catch((e) => setError(e.message));
            }}
          >
            <option value="">Select channel</option>
            {channels.map((c) => (
              <option key={c.id} value={c.id}>
                {c.number} · {c.name}
              </option>
            ))}
          </select>
        </label>
        {branding && (
          <>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void act(() => api.saveBranding(branding, csrf));
              }}
            >
              <div className="presentation-fields">
                {(["enabled", "inherit", "bug"] as const).map((key) => (
                  <label key={key}>
                    <input
                      type="checkbox"
                      checked={branding[key]}
                      onChange={(e) =>
                        setBranding({ ...branding, [key]: e.target.checked })
                      }
                    />
                    {key === "enabled"
                      ? "Enable interstitials"
                      : key === "inherit"
                        ? "Inherit global rules"
                        : "Channel bug"}
                  </label>
                ))}
                <label>
                  Compact channel logo
                  <input
                    type="file"
                    accept="image/png,image/jpeg,image/webp"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file)
                        void channelAPI
                          .upload(file, csrf)
                          .then((v) =>
                            setBranding({ ...branding, compactLogoId: v.id }),
                          )
                          .catch((e) => setError(e.message));
                    }}
                  />
                </label>
                <button
                  type="button"
                  onClick={() =>
                    setBranding({ ...branding, compactLogoId: "" })
                  }
                >
                  Use primary logo
                </button>
                <label>
                  Bug corner
                  <select
                    value={branding.corner}
                    onChange={(e) =>
                      setBranding({ ...branding, corner: e.target.value })
                    }
                  >
                    {[
                      "top-left",
                      "top-right",
                      "bottom-left",
                      "bottom-right",
                    ].map((c) => (
                      <option key={c}>{c}</option>
                    ))}
                  </select>
                </label>
                <label>
                  Bug opacity
                  <input
                    type="number"
                    min="0.1"
                    max="0.7"
                    step="0.05"
                    value={branding.opacity}
                    onChange={(e) =>
                      setBranding({ ...branding, opacity: +e.target.value })
                    }
                  />
                </label>
                <label>
                  Bug size
                  <input
                    type="number"
                    min="32"
                    max="128"
                    value={branding.size}
                    onChange={(e) =>
                      setBranding({ ...branding, size: +e.target.value })
                    }
                  />
                </label>
                <label>
                  Coming Up Next seconds (0 disables)
                  <input
                    type="number"
                    min="0"
                    max="120"
                    value={branding.nextUpSeconds}
                    onChange={(e) =>
                      setBranding({
                        ...branding,
                        nextUpSeconds: +e.target.value,
                      })
                    }
                  />
                </label>
              </div>
              <button>Save channel presentation</button>
            </form>
            <div className="branding-preview" aria-label="Branding preview">
              <BroadcastOverlay
                tune={sample}
                previewLogo={
                  branding.compactLogoId
                    ? `/api/v1/channel-artwork/${branding.compactLogoId}`
                    : undefined
                }
              />
            </div>
            <p>Preview does not change the live schedule.</p>
            <button
              onClick={() => {
                if (
                  confirm(
                    "Repair future programming at the next safe boundary? Current programming is protected.",
                  )
                )
                  void act(
                    () =>
                      request(
                        `/channels/${branding.channelId}/schedule/repair`,
                        "POST",
                        {},
                        csrf,
                      ),
                    "Future repair queued. Inspect Scheduling Studio for the complete timeline.",
                  );
              }}
            >
              Repair future schedule
            </button>
            {rules
              .filter((r) => !r.channelId)
              .map((r) => (
                <div key={r.id}>
                  {r.name}
                  <button
                    onClick={() =>
                      void act(() =>
                        request(
                          `/channels/${branding.channelId}/presentation/rules/${r.id}`,
                          "PUT",
                          { enabled: false },
                          csrf,
                        ),
                      )
                    }
                  >
                    Disable for this channel
                  </button>
                  <button
                    onClick={() =>
                      void act(() =>
                        request(
                          `/channels/${branding.channelId}/presentation/rules/${r.id}`,
                          "PUT",
                          { enabled: true },
                          csrf,
                        ),
                      )
                    }
                  >
                    Enable for this channel
                  </button>
                </div>
              ))}
          </>
        )}
      </section>
    </div>
  );
}
