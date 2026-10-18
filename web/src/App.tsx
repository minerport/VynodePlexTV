import { PresentationAdmin } from "./presentation/PresentationAdmin";
import { MyChannels } from "./personalization/MyChannels";
import { ForYou } from "./personalization/ForYou";
import { GuideGrid } from "./guide/GuideGrid";
import { watch } from "./playback/api";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
  lazy,
  Suspense,
} from "react";
import {
  ArrowRight,
  Check,
  ChevronRight,
  CircleHelp,
  Clapperboard,
  Grid2X2,
  Layers,
  LogOut,
  Radio,
  Settings2,
  ShieldCheck,
  Sparkles,
  Tv,
  Wifi,
} from "lucide-react";
import { bootstrap, request, type Session, type Settings } from "./api";
import { useTVFocus } from "./focus";
import { PlexPage } from "./PlexPage";
import { ChannelsPage } from "./channels/ChannelsPage";
const Player = lazy(() =>
  import("./playback/Player").then((m) => ({ default: m.Player })),
);
import { ViewerHome, ActiveViewers } from "./playback/ViewerHome";

type Page =
  | "live"
  | "guide"
  | "channels"
  | "plex"
  | "settings"
  | "presentation"
  | "my-channels"
  | "whats-good";
const pages: { id: Page; name: string; icon: typeof Tv }[] = [
  { id: "live", name: "Live TV", icon: Tv },
  { id: "guide", name: "Guide", icon: Grid2X2 },
  { id: "my-channels", name: "My Channels", icon: Layers },
  { id: "whats-good", name: "What’s Good", icon: Sparkles },
  { id: "channels", name: "Channels", icon: Layers },
  { id: "plex", name: "Plex Setup", icon: Radio },
  { id: "presentation", name: "Broadcast", icon: Clapperboard },
  { id: "settings", name: "Settings", icon: Settings2 },
];
const getPage = (): Page =>
  pages.find((p) => `#${p.id}` === window.location.hash)?.id ?? "live";
const message = (error: unknown) =>
  error instanceof Error
    ? error.message
    : "Something went wrong. Please try again.";

function Brand() {
  return (
    <div className="brand">
      <span className="brand-symbol">
        <Tv size={24} />
      </span>
      <span>
        afterglow<span className="brand-tv">TV</span>
      </span>
    </div>
  );
}

export function App() {
  const [state, setState] = useState<
    "loading" | "setup" | "login" | "ready" | "error"
  >("loading");
  const [session, setSession] = useState<Session | null>(null);
  const [error, setError] = useState("");
  const initialize = useCallback(
    () =>
      bootstrap()
        .then((result) => {
          setSession(result.session);
          setState(result.state);
        })
        .catch((e) => {
          setError(message(e));
          setState("error");
        }),
    [],
  );
  useEffect(() => {
    void initialize();
  }, [initialize]);
  useEffect(() => {
    const expire = () => {
      setSession(null);
      setState("login");
    };
    window.addEventListener("session-expired", expire);
    return () => window.removeEventListener("session-expired", expire);
  }, []);
  if (state === "ready" && session)
    return (
      <ViewingShell
        session={session}
        onLogout={() => {
          setSession(null);
          setState("login");
        }}
      />
    );
  return (
    <div className="welcome">
      <div className="welcome-art">
        <Brand />
        <div className="welcome-copy">
          <span className="eyebrow">YOUR LIBRARY. A WHOLE NEW RHYTHM.</span>
          <h1>
            Less searching.
            <br />
            More <em>watching.</em>
          </h1>
          <p>
            A place for your favorites to come together.
            <br />
            Made for the big screen. Made to feel like television.
          </p>
          <div className="welcome-tags">
            <span>
              <Tv size={16} /> TV first
            </span>
            <span>
              <ShieldCheck size={16} /> Entirely self-hosted
            </span>
          </div>
        </div>
        <span className="welcome-foot">
          THE NEXT CHAPTER OF YOUR HOME CINEMA
        </span>
      </div>
      <div className="welcome-form">
        {state === "loading" ? (
          <div role="status" className="loading">
            Opening Afterglow…
          </div>
        ) : state === "error" ? (
          <div role="alert">
            <h2>Let’s reconnect</h2>
            <p>{error}</p>
            <button
              onClick={() => {
                setState("loading");
                setError("");
                void initialize();
              }}
            >
              Try again
            </button>
          </div>
        ) : (
          <AuthForm
            setup={state === "setup"}
            onCreated={() => setState("login")}
            onLogin={(s) => {
              setSession(s);
              setState("ready");
            }}
          />
        )}
        <div className="foundation-note">
          YOUR SELF-HOSTED TELEVISION
          <br />
          <span>
            Connect Plex now. Live programming arrives in later phases.
          </span>
        </div>
      </div>
    </div>
  );
}

function AuthForm({
  setup,
  onCreated,
  onLogin,
}: {
  setup: boolean;
  onCreated: () => void;
  onLogin: (s: Session) => void;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState(false);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (setup) {
        await request("/auth/setup", "POST", { username, password });
        setPassword("");
        setCreated(true);
        onCreated();
      } else
        onLogin(
          await request<Session>("/auth/login", "POST", { username, password }),
        );
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <form onSubmit={submit} className="auth-form">
      <span className="eyebrow">
        {setup ? "WELCOME TO AFTERGLOW" : "YOUR SEAT IS WAITING"}
      </span>
      <h2>{setup ? "Make yourself at home." : "Welcome back."}</h2>
      <p>
        {setup
          ? "Create your owner account to start your own television experience."
          : "Sign in to your television space."}
      </p>
      {created && (
        <div className="success" role="status">
          Owner account created. Sign in to continue.
        </div>
      )}
      <label>
        Username
        <input
          autoFocus
          autoComplete="username"
          required
          minLength={3}
          maxLength={40}
          pattern="[A-Za-z0-9._\-]+"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
        />
      </label>
      <label>
        Password
        <input
          type="password"
          autoComplete={setup ? "new-password" : "current-password"}
          required
          minLength={setup ? 12 : 1}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
      </label>
      {setup && (
        <span className="field-help">
          Use 12–72 bytes. Longer passphrases work well.
        </span>
      )}
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <button className="primary" disabled={busy}>
        {busy ? "One moment…" : setup ? "Create owner account" : "Sign in"}
        <ArrowRight size={18} />
      </button>
      <span className="private-note">
        <ShieldCheck size={15} /> Your account stays on this server.
      </span>
    </form>
  );
}

function ViewingShell(props: { session: Session; onLogout: () => void }) {
  const [hash, setHash] = useState(window.location.hash);
  useEffect(() => {
    const onHash = () => setHash(window.location.hash);
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);
  if (hash.startsWith("#watch/")) {
    const [id, query] = hash.slice(7).split("?");
    const programId = new URLSearchParams(query).get("program") ?? undefined;
    return (
      <Suspense
        fallback={
          <div className="tv-player" role="status">
            Opening television…
          </div>
        }
      >
        <Player
          channelId={id}
          programId={programId}
          userId={props.session.user.id}
          csrf={props.session.csrfToken}
          onExit={() => {
            window.location.hash =
              sessionStorage.getItem("afterglow-return") || "live";
          }}
        />
      </Suspense>
    );
  }
  return <Shell {...props} />;
}
function Shell({
  session,
  onLogout,
}: {
  session: Session;
  onLogout: () => void;
}) {
  const [page, setPage] = useState<Page>(getPage);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const mainRef = useRef<HTMLElement>(null);
  const navigate = useCallback((next: Page) => {
    window.location.hash = next;
    setPage(next);
    requestAnimationFrame(() => mainRef.current?.focus());
  }, []);
  const back = useCallback(() => navigate("live"), [navigate]);
  useTVFocus(back);
  useEffect(() => {
    const change = () => setPage(getPage());
    window.addEventListener("hashchange", change);
    return () => window.removeEventListener("hashchange", change);
  }, []);
  useEffect(() => {
    request<Settings>("/settings")
      .then(setSettings)
      .catch((e) => setError(message(e)));
  }, []);
  async function logout() {
    setBusy(true);
    try {
      await request("/auth/logout", "POST", undefined, session.csrfToken);
      onLogout();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="app-shell">
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header>
        <Brand />
        <nav aria-label="Main navigation">
          {pages
            .filter(
              (p) =>
                session.user.role !== "USER" ||
                !["channels", "plex", "presentation"].includes(p.id),
            )
            .map(({ id, name, icon: Icon }) => (
              <a
                key={id}
                href={`#${id}`}
                aria-current={page === id ? "page" : undefined}
                onClick={(e) => {
                  e.preventDefault();
                  navigate(id);
                }}
              >
                <Icon size={18} />
                <span>{name}</span>
              </a>
            ))}
        </nav>
        <div className="profile">
          <span className="avatar">
            {session.user.username.slice(0, 1).toUpperCase()}
          </span>
          <span>
            {session.user.username}
            <small>{session.user.role.toLowerCase()}</small>
          </span>
          <button
            className="icon-button"
            aria-label="Sign out"
            title="Sign out"
            disabled={busy}
            onClick={() => void logout()}
          >
            <LogOut size={17} />
          </button>
        </div>
      </header>
      <main id="main" ref={mainRef} tabIndex={-1}>
        {error && (
          <div className="error" role="alert">
            {error}
          </div>
        )}
        {page === "live" ? (
          <ViewerHome
            userId={session.user.id}
            csrf={session.csrfToken}
            fallback={<Live navigate={navigate} />}
          />
        ) : page === "guide" ? (
          <GuideGrid
            csrf={session.csrfToken}
            userId={session.user.id}
            onTune={watch}
          />
        ) : page === "my-channels" ? (
          <MyChannels csrf={session.csrfToken} />
        ) : page === "whats-good" ? (
          <div className="viewer-home">
            <ForYou csrf={session.csrfToken} expanded />
          </div>
        ) : page === "presentation" ? (
          <PresentationAdmin session={session} />
        ) : page === "settings" ? (
          <>
            <SettingsPage
              settings={settings}
              onSave={setSettings}
              session={session}
            />
            {session.user.role !== "USER" && <ActiveViewers />}
          </>
        ) : page === "plex" ? (
          <PlexPage session={session} />
        ) : page === "channels" ? (
          <ChannelsPage session={session} />
        ) : (
          <EmptyPage page={page} navigate={navigate} />
        )}
      </main>
      <footer>
        <span>
          <span className="status-dot" />{" "}
          {settings?.displayName ?? "Afterglow TV"}{" "}
          <span className="footer-divider">/</span> Live television
        </span>
        <span className="key-hints">
          <kbd>↑</kbd>
          <kbd>↓</kbd> Navigate <kbd>Enter</kbd> Select <kbd>Esc</kbd> Home
        </span>
      </footer>
    </div>
  );
}

function Live({ navigate }: { navigate: (p: Page) => void }) {
  return (
    <>
      <section className="hero">
        <div className="hero-art" aria-hidden="true">
          <div className="orbit orbit-one" />
          <div className="orbit orbit-two" />
          <div className="horizon" />
          <div className="art-caption">AFTER HOURS. ON YOUR TERMS.</div>
        </div>
        <div className="hero-content">
          <div className="pill">
            <span className="status-dot" /> YOUR PERSONAL TELEVISION
          </div>
          <h1>
            Great television.
            <br />
            <em>Your collection.</em>
          </h1>
          <p>
            Your favorite films. The shows you return to.
            <br />A familiar feeling, with something new on next.
          </p>
          <div className="hero-actions">
            <button className="primary" onClick={() => navigate("plex")}>
              <Radio size={19} /> Explore Plex setup <ArrowRight size={18} />
            </button>
            <button className="secondary" onClick={() => navigate("guide")}>
              Open guide
            </button>
          </div>
          <span className="hero-note">
            Your screen is ready. Live channels arrive in a later phase.
          </span>
        </div>
        <div className="hero-bottom">
          <span>
            <Sparkles size={16} /> THE ART OF JUST TUNING IN
          </span>
          <span>
            01 <span className="small-line" /> A NEW BEGINNING
          </span>
        </div>
      </section>
      <section className="below-hero">
        <div className="section-title">
          <div>
            <span className="eyebrow">MAKE IT YOURS</span>
            <h2>Your television starts here</h2>
          </div>
          <span className="muted">
            A little setup. A lot to look forward to.
          </span>
        </div>
        <div className="journey">
          <button className="journey-card" onClick={() => navigate("plex")}>
            <span className="card-icon">
              <Radio />
            </span>
            <span className="step">01 / CONNECT</span>
            <h3>Bring your library</h3>
            <p>
              Plex will be the source of your
              <br />
              personal television universe.
            </p>
            <span className="card-link">
              Plex setup <ChevronRight size={17} />
            </span>
          </button>
          <button className="journey-card" onClick={() => navigate("channels")}>
            <span className="card-icon violet">
              <Layers />
            </span>
            <span className="step">02 / CURATE</span>
            <h3>Find your frequency</h3>
            <p>
              Dedicated shows, movie nights,
              <br />
              and channels that feel like you.
            </p>
            <span className="card-link">
              Your channels <ChevronRight size={17} />
            </span>
          </button>
          <button className="journey-card" onClick={() => navigate("guide")}>
            <span className="card-icon blue">
              <Grid2X2 />
            </span>
            <span className="step">03 / TUNE IN</span>
            <h3>Let the evening unfold</h3>
            <p>
              One guide. A world of favorites.
              <br />
              Something good, already on.
            </p>
            <span className="card-link">
              Program guide <ChevronRight size={17} />
            </span>
          </button>
        </div>
      </section>
    </>
  );
}

function EmptyPage({
  page,
  navigate,
}: {
  page: Exclude<
    Page,
    "live" | "settings" | "presentation" | "my-channels" | "whats-good"
  >;
  navigate: (p: Page) => void;
}) {
  const content = {
    guide: {
      eyebrow: "THE PROGRAM GUIDE",
      title: "An evening of possibilities.",
      description:
        "Your channels and their programming will meet here. Browse across time, find a favorite, and join right where the story is.",
      icon: Grid2X2,
      empty: "No programming yet",
      note: "The guide becomes available after Plex, channels, and scheduling are implemented.",
    },
    channels: {
      eyebrow: "YOUR CHANNEL COLLECTION",
      title: "A frequency for every mood.",
      description:
        "Movie marathons. Comfort shows. A whole channel for your favorite universe. This will be television, your way.",
      icon: Layers,
      empty: "Your channel collection is waiting",
      note: "Channel creation arrives in Phase 2, after real Plex library integration.",
    },
    plex: {
      eyebrow: "THE SOURCE OF IT ALL",
      title: "Your library. Center stage.",
      description:
        "Your Plex servers will provide the films, episodes, and artwork that make Afterglow yours. Your media stays with Plex.",
      icon: Radio,
      empty: "Plex connection is coming next",
      note: "Phase 1 will add secure server connections, library selection, and metadata synchronization.",
    },
  }[page];
  const Icon = content.icon;
  return (
    <section className="destination">
      <div className="destination-heading">
        <span className="eyebrow">{content.eyebrow}</span>
        <h1>{content.title}</h1>
        <p>{content.description}</p>
      </div>
      <div className="empty-panel">
        <div className="empty-symbol">
          <Icon size={36} />
        </div>
        <span className="pill">FOUNDATION PREVIEW</span>
        <h2>{content.empty}</h2>
        <p>{content.note}</p>
        <button className="secondary" onClick={() => navigate("live")}>
          Back to Live TV <ArrowRight size={17} />
        </button>
      </div>
      <div className="info-strip">
        <CircleHelp size={18} />
        <span>
          {page === "plex"
            ? "No server credentials are requested or stored in this phase."
            : "There are no sample channels or simulated programs. Your own library will bring this space to life."}
        </span>
      </div>
    </section>
  );
}

function SettingsPage({
  settings,
  onSave,
  session,
}: {
  settings: Settings | null;
  onSave: (s: Settings) => void;
  session: Session;
}) {
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const [busy, setBusy] = useState(false);
  const canEdit = session.user.role !== "USER";
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setBusy(true);
    setSaved(false);
    setError("");
    try {
      const value = await request<Settings>(
        "/settings",
        "PUT",
        {
          displayName: data.get("displayName"),
          timezone: data.get("timezone"),
        },
        session.csrfToken,
      );
      onSave(value);
      setSaved(true);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="destination">
      <div className="destination-heading">
        <span className="eyebrow">A SPACE THAT’S YOURS</span>
        <h1>Set the atmosphere.</h1>
        <p>The essentials for your television space.</p>
      </div>
      <div className="settings-layout">
        <form className="settings-panel" onSubmit={save}>
          <div className="panel-title">
            <Settings2 size={22} />
            <h2>General settings</h2>
          </div>
          {!settings ? (
            <p role="status">Settings have not loaded. Refresh to try again.</p>
          ) : (
            <>
              <label>
                Television name
                <input
                  name="displayName"
                  defaultValue={settings.displayName}
                  maxLength={60}
                  required
                  disabled={!canEdit}
                />
              </label>
              <label>
                Guide timezone
                <input
                  name="timezone"
                  defaultValue={settings.timezone}
                  placeholder="America/New_York"
                  required
                  disabled={!canEdit}
                />
              </label>
              <p className="field-help">
                Use an IANA timezone, such as America/New_York or Europe/London.
                Future guide times will use this setting.
              </p>
              {canEdit && (
                <button className="primary" disabled={busy}>
                  {busy ? "Saving…" : "Save settings"}
                  <Check size={18} />
                </button>
              )}
            </>
          )}
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          {saved && (
            <p className="success" role="status">
              Settings saved.
            </p>
          )}
        </form>
        <aside className="settings-aside">
          <ShieldCheck size={28} />
          <h3>At home on your server.</h3>
          <p>
            Your account and preferences are stored locally. No cloud account
            required.
          </p>
          <div>
            <Wifi size={16} /> Self-hosted foundation
          </div>
          <div>
            <Clapperboard size={16} /> Plex integration · Phase 1
          </div>
          <div>
            <Check size={16} /> Signed in as {session.user.role.toLowerCase()}
          </div>
        </aside>
      </div>
    </section>
  );
}
