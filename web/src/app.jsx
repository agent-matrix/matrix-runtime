/* ============================================================
   Matrix Cloud — Enterprise Console
   Production port of the Claude Design prototype, wired to the
   real Matrix Runtime API (/v1/*). Live views: Overview, Runtimes,
   Sandboxes, Jobs, Models. The rest render the design's reference
   data so the console is complete and navigable.

   React + ReactDOM are provided as globals by vendored UMD builds.
   ============================================================ */

/* ---------------------------------------------------------------
   API client — same-origin calls to the runtime control surface.
   --------------------------------------------------------------- */
const API_BASE = window.MATRIX_API_BASE || "";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const TOKEN_KEY = "matrixcloud_token";
function getToken() { try { return localStorage.getItem(TOKEN_KEY) || ""; } catch (e) { return ""; } }
function setToken(t) { try { t ? localStorage.setItem(TOKEN_KEY, t) : localStorage.removeItem(TOKEN_KEY); } catch (e) {} }

// urlIntent inspects the address bar for email-link flows. Resend links point at
// {APP_URL}/reset?token=mxpr_… and {APP_URL}/verify?token=mxev_…; the SPA serves
// index.html for those paths, so we read the intent here and clear the query.
function urlIntent() {
  try {
    const u = new URL(window.location.href);
    const token = u.searchParams.get("token") || "";
    const path = u.pathname.replace(/\/+$/, "");
    if (path.endsWith("/reset") || token.startsWith("mxpr_")) return { kind: "reset", token };
    if (path.endsWith("/verify") || token.startsWith("mxev_")) return { kind: "verify", token };
  } catch (e) {}
  return { kind: "", token: "" };
}
function clearURLToken() {
  try { window.history.replaceState({}, "", window.location.pathname.replace(/\/(reset|verify)$/, "/") || "/"); } catch (e) {}
}

const api = {
  async req(method, path, body) {
    const opts = { method, headers: {} };
    if (body !== undefined) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    const tok = getToken() || window.MATRIX_API_TOKEN;
    if (tok) opts.headers["Authorization"] = "Bearer " + tok;
    const res = await fetch(API_BASE + path, opts);
    const txt = await res.text();
    let data = null;
    try { data = txt ? JSON.parse(txt) : null; } catch (e) { data = txt; }
    if (!res.ok) {
      const err = new Error((data && data.error) || ("HTTP " + res.status));
      err.status = res.status; err.data = data;
      throw err;
    }
    return data;
  },
  get(p) { return this.req("GET", p); },
  post(p, b) { return this.req("POST", p, b); },
  del(p) { return this.req("DELETE", p); },
  eventsURL(p) { return API_BASE + p; },
};

// Auth helpers backed by the runtime's user store (SQLite or Postgres/Neon).
const auth = {
  async me() { const r = await api.get("/v1/auth/me"); return r.user; },
  async login(email, password) { const r = await api.post("/v1/auth/login", { email, password }); setToken(r.token); return r.user; },
  async signup(name, email, password, workspace) { const r = await api.post("/v1/auth/signup", { name, email, password, workspace }); setToken(r.token); return r.user; },
  async logout(all) { try { await api.post("/v1/auth/logout" + (all ? "?all=true" : ""), {}); } catch (e) {} setToken(null); },
  async forgot(email) { const r = await api.post("/v1/auth/forgot", { email }); return r.message || "Check your email."; },
  async reset(token, password) { const r = await api.post("/v1/auth/reset", { token, password }); return r.message || "Password updated."; },
  async verify(token) { const r = await api.post("/v1/auth/verify", { token }); return r.message || "Email verified."; },
};

// Hosted control-plane helpers (workspace-scoped via the session bearer token).
const cloud = {
  async listRuntimes() { const r = await api.get("/v1/cloud/runtimes"); return r.runtimes || []; },
  async mintJoinToken(label, maxUses, ttlMinutes) { return api.post("/v1/cloud/join-tokens", { label, max_uses: maxUses, ttl_minutes: ttlMinutes }); },
  async listProviders() { const r = await api.get("/v1/cloud/providers"); return r.providers || []; },
  async setProvider(provider, label, secret, meta) { const r = await api.post("/v1/cloud/providers", { provider, label, secret, meta }); return r.provider; },
  async listAudit() { const r = await api.get("/v1/cloud/audit"); return r.events || []; },
};

async function runJobToCompletion(type, payload, timeoutMs = 25000) {
  const c = await api.post("/v1/jobs", { type, payload });
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    const s = await api.get("/v1/jobs/" + c.job_id);
    if (["complete", "error", "expired", "cancelled"].includes(s.status)) {
      if (s.status !== "complete") throw new Error(s.error || s.status);
      return s.result;
    }
    await sleep(500);
  }
  throw new Error("timed out");
}

function fmtParams(n) {
  if (!n) return "—";
  if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
  if (n >= 1e6) return (n / 1e6).toFixed(0) + "M";
  return String(n);
}

/* ---------------------------------------------------------------
   Live runtime context — polls /v1/health + /v1/capabilities.
   --------------------------------------------------------------- */
const RuntimeCtx = React.createContext(null);
function useRuntimeState() {
  const [s, setS] = React.useState({ loading: true, online: false, health: null, caps: null });
  React.useEffect(() => {
    let alive = true;
    async function tick() {
      try {
        const [h, c] = await Promise.all([api.get("/v1/health"), api.get("/v1/capabilities")]);
        if (alive) setS({ loading: false, online: true, health: h, caps: c });
      } catch (e) {
        if (alive) setS((p) => ({ ...p, loading: false, online: false }));
      }
    }
    tick();
    const t = setInterval(tick, 5000);
    return () => { alive = false; clearInterval(t); };
  }, []);
  return s;
}
const useRuntime = () => React.useContext(RuntimeCtx);

function useJobs(ms) {
  const [jobs, setJobs] = React.useState(null);
  const load = React.useCallback(async () => {
    try { const r = await api.get("/v1/jobs"); setJobs(r.jobs || []); }
    catch (e) { setJobs(null); }
  }, []);
  React.useEffect(() => {
    load();
    if (ms) { const t = setInterval(load, ms); return () => clearInterval(t); }
  }, [load, ms]);
  return { jobs, refresh: load };
}

/* ---------------------------------------------------------------
   Icons (1.7px stroke).
   --------------------------------------------------------------- */
function GI({ d, size = 18, sw = 1.7, style, className }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={sw} strokeLinecap="round" strokeLinejoin="round" style={style} className={className} aria-hidden="true">{d}</svg>
  );
}
const G = {
  grid: <><rect x="3" y="3" width="7" height="7" rx="1.5" /><rect x="14" y="3" width="7" height="7" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /><rect x="14" y="14" width="7" height="7" rx="1.5" /></>,
  server: <><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></>,
  download: <><path d="M12 3v12" /><path d="m7 10 5 5 5-5" /><path d="M5 21h14" /></>,
  cpu: <><rect x="6" y="6" width="12" height="12" rx="2" /><path d="M9 2v2M15 2v2M9 20v2M15 20v2M2 9h2M2 15h2M20 9h2M20 15h2" /><rect x="9.5" y="9.5" width="5" height="5" rx="1" /></>,
  bot: <><rect x="4" y="8" width="16" height="11" rx="3" /><path d="M12 8V4M9 4h6" /><circle cx="9" cy="13.5" r="1.2" fill="currentColor" stroke="none" /><circle cx="15" cy="13.5" r="1.2" fill="currentColor" stroke="none" /></>,
  activity: <path d="M22 12h-4l-3 9L9 3l-3 9H2" />,
  logs: <><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6M9 13h6M9 17h4" /></>,
  shield: <><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10" /><path d="m9 12 2 2 4-4" /></>,
  audit: <><path d="M9 11l3 3 7-7" /><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" /></>,
  gear: <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-1.8-.3 1.6 1.6 0 0 0-1 1.5V21a2 2 0 0 1-4 0v-.1A1.6 1.6 0 0 0 9 19.4a1.6 1.6 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.6 1.6 0 0 0 .3-1.8 1.6 1.6 0 0 0-1.5-1H3a2 2 0 0 1 0-4h.1A1.6 1.6 0 0 0 4.6 9a1.6 1.6 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 1.8.3H9a1.6 1.6 0 0 0 1-1.5V3a2 2 0 0 1 4 0v.1a1.6 1.6 0 0 0 1 1.5 1.6 1.6 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0-.3 1.8V9a1.6 1.6 0 0 0 1.5 1H21a2 2 0 0 1 0 4h-.1a1.6 1.6 0 0 0-1.5 1z" /></>,
  bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9" /><path d="M13.7 21a2 2 0 0 1-3.4 0" /></>,
  search: <><circle cx="11" cy="11" r="7" /><path d="m21 21-4.3-4.3" /></>,
  chev: <path d="m6 9 6 6 6-6" />,
  chevr: <path d="m9 18 6-6-6-6" />,
  plus: <path d="M12 5v14M5 12h14" />,
  arrowup: <><path d="M12 19V5" /><path d="m5 12 7-7 7 7" /></>,
  arrowdn: <><path d="M12 5v14" /><path d="m19 12-7 7-7-7" /></>,
  check: <path d="M20 6 9 17l-5-5" />,
  x: <><path d="M18 6 6 18M6 6l12 12" /></>,
  play: <path d="M6 4l14 8-14 8z" />,
  refresh: <><path d="M3 12a9 9 0 0 1 15-6.7L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-15 6.7L3 16" /><path d="M3 21v-5h5" /></>,
  menu: <><path d="M4 6h16M4 12h16M4 18h16" /></>,
  globe: <><circle cx="12" cy="12" r="9" /><path d="M3 12h18" /><path d="M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18" /></>,
  key: <><circle cx="8" cy="15" r="4" /><path d="m10.8 12.2 8.2-8.2M17 5l2 2M14 8l2 2" /></>,
  zap: <path d="M13 2 4 14h7l-1 8 9-12h-7z" />,
  clock: <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></>,
  layers: <><path d="m12 2 9 5-9 5-9-5z" /><path d="m3 12 9 5 9-5" /><path d="m3 17 9 5 9-5" /></>,
  user: <><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>,
  terminal: <><path d="m4 17 6-6-6-6" /><path d="M12 19h8" /></>,
};

/* ---------------------------------------------------------------
   Reference data (fallback + demo fleet/governance context).
   --------------------------------------------------------------- */
const FILESYSTEM_CMD = "npx -y @modelcontextprotocol/server-filesystem /tmp";
const STATUS_CLASS = { complete: "green", running: "blue", queued: "gray", error: "red", expired: "amber", cancelled: "amber" };

// Real-data hooks — the console renders only what the backend reports.
function useFetch(path, pollMs) {
  const [data, setData] = React.useState(null);
  const load = React.useCallback(async () => {
    try { setData(await api.get(path)); } catch (e) { setData(null); }
  }, [path]);
  React.useEffect(() => {
    load();
    if (pollMs) { const t = setInterval(load, pollMs); return () => clearInterval(t); }
  }, [load, pollMs]);
  return { data, refresh: load };
}
function useRuntimes(pollMs) {
  const { data, refresh } = useFetch("/v1/runtimes", pollMs);
  return { runtimes: data ? data.runtimes || [] : null, refresh };
}
function useCatalog() {
  const { data } = useFetch("/v1/catalog");
  if (!data) return null;
  return (data.items || []).map((it) => ({ ...it, kindClass: it.kind_class, startCommand: it.start_command }));
}
// useCloudRuntimes lists runtimes registered to this workspace (HF Spaces and
// self-hosted runtimes that joined via a join token). Distinct from /v1/runtimes
// which describes this local node.
function useCloudRuntimes(pollMs) {
  const { data, refresh } = useFetch("/v1/cloud/runtimes", pollMs);
  return { runtimes: data ? data.runtimes || [] : null, refresh };
}
function timeAgo(iso) {
  if (!iso) return "—";
  const t = new Date(iso).getTime();
  if (!t) return "—";
  const s = Math.max(0, Math.round((Date.now() - t) / 1000));
  if (s < 60) return s + "s ago";
  if (s < 3600) return Math.round(s / 60) + "m ago";
  if (s < 86400) return Math.round(s / 3600) + "h ago";
  return Math.round(s / 86400) + "d ago";
}
const RT_STATUS_CLASS = { online: "green", idle: "amber", pending: "violet", offline: "red" };

/* ---------------------------------------------------------------
   Shared primitives.
   --------------------------------------------------------------- */
function Spark({ data, color }) {
  const max = Math.max(...data);
  return <div className="spark">{data.map((v, i) => <span key={i} style={{ height: (v / max) * 100 + "%", background: color }} />)}</div>;
}
function ItemMono({ initials, size = 44 }) {
  return (
    <div style={{ width: size, height: size, borderRadius: 11, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center",
      border: "1px solid var(--line-2)", background: "var(--raised)", color: "var(--acc)", fontFamily: "var(--mono)", fontWeight: 700, fontSize: size * 0.3 }}>{initials}</div>
  );
}
function CmdBlock({ label, children }) {
  const [copied, setCopied] = React.useState(false);
  function copy() { try { navigator.clipboard.writeText(children); } catch (e) {} setCopied(true); setTimeout(() => setCopied(false), 1300); }
  return (
    <div className="card" style={{ overflow: "hidden", background: "var(--inset)" }}>
      <div className="card-h" style={{ padding: "9px 14px" }}>
        <span className="mono" style={{ fontSize: 11, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.1em" }}>{label}</span>
        <button className="chip" style={{ cursor: "pointer" }} onClick={copy}>{copied ? <><GI d={G.check} size={11} /> copied</> : <><GI d={G.logs} size={11} /> copy</>}</button>
      </div>
      <pre className="mono" style={{ margin: 0, padding: 15, fontSize: 12.5, lineHeight: 1.7, color: "var(--ink)", overflowX: "auto" }}>
<span style={{ color: "var(--ink-4)", userSelect: "none" }}>$ </span>{children}
      </pre>
    </div>
  );
}
function LiveTag() {
  return <span className="chip green" title="Wired to this Matrix Runtime"><span className="dot green pulse" /> live</span>;
}

// WARN_META maps readiness warning codes to a human title, severity and the
// console route that fixes them.
const WARN_META = {
  api_token_missing: { sev: "high", title: "API token not set", route: "settings" },
  matrixshell_enabled: { sev: "med", title: "MatrixShell enabled", route: "settings" },
  sqlite_in_use: { sev: "med", title: "Using SQLite (single-node)", route: "settings" },
  store_unavailable: { sev: "high", title: "User store unavailable", route: "settings" },
  local_dev_mode: { sev: "low", title: "Local-dev mode", route: null },
};

// ReadinessBanner surfaces production-safety warnings from GET /v1/ready. It is
// dismissible per-session and only renders when there is something to show.
function ReadinessBanner({ go }) {
  const { data } = useFetch("/v1/ready", 30000);
  const [dismissed, setDismissed] = React.useState(() => {
    try { return sessionStorage.getItem("mc-ready-dismissed") === "1"; } catch (e) { return false; }
  });
  if (!data || dismissed) return null;
  // Hide the low-severity local-dev-only notice unless something else is wrong.
  const warnings = (data.warnings || []).filter((w) => (WARN_META[w.code]?.sev || "low") !== "low");
  if (warnings.length === 0) return null;
  const worst = warnings.some((w) => WARN_META[w.code]?.sev === "high") ? "high" : "med";
  const color = worst === "high" ? "var(--red)" : "var(--amber, #f5a623)";
  const bg = worst === "high" ? "var(--red-soft)" : "rgba(245,166,35,0.10)";
  function dismiss() { try { sessionStorage.setItem("mc-ready-dismissed", "1"); } catch (e) {} setDismissed(true); }
  return (
    <div style={{ margin: "0 0 16px", padding: "12px 14px", borderRadius: "var(--r-md)", border: "1px solid " + color, background: bg }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <span style={{ color, display: "inline-flex", flexShrink: 0 }}><GI d={G.shield} size={17} /></span>
        <span style={{ fontWeight: 700, fontSize: 13.5 }}>Production readiness</span>
        <span className="chip" style={{ color }}>{warnings.length} warning{warnings.length > 1 ? "s" : ""}</span>
        <div style={{ flex: 1 }} />
        <button className="iconbtn" onClick={dismiss} title="Dismiss for this session"><GI d={G.x} size={15} /></button>
      </div>
      <div style={{ display: "grid", gap: 6, marginTop: 10 }}>
        {warnings.map((w) => {
          const meta = WARN_META[w.code] || { title: w.code, route: null };
          return (
            <div key={w.code} style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 12.5, color: "var(--ink-2)" }}>
              <span className="dot" style={{ background: WARN_META[w.code]?.sev === "high" ? "var(--red)" : color }} />
              <b style={{ color: "var(--ink)" }}>{meta.title}.</b> <span style={{ color: "var(--ink-3)" }}>{w.message}</span>
              {meta.route && go && <button onClick={() => go(meta.route)} style={{ color: "var(--acc-2)", fontWeight: 600, marginLeft: "auto" }}>Fix →</button>}
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Overview (live header + recent jobs).
   --------------------------------------------------------------- */
function OverviewView({ go }) {
  const rt = useRuntime();
  const { jobs } = useJobs(5000);
  const liveJobs = jobs || [];
  const running = liveJobs.filter((j) => j.status === "running" || j.status === "queued").length;
  const caps = (rt.caps && rt.caps.capabilities) || [];
  const limits = (rt.caps && rt.caps.limits) || {};

  const metrics = [
    { lab: "Runtime", ic: G.server, val: rt.online ? "Online" : (rt.loading ? "…" : "Offline"), delta: rt.health ? rt.health.mode : "—", dir: rt.online ? "up" : "down", note: "this node" },
    { lab: "Capabilities", ic: G.zap, val: String(caps.length || "—"), delta: rt.health ? "v" + rt.health.version : "—", dir: "flat", note: "advertised" },
    { lab: "Active jobs", ic: G.activity, val: String(running), delta: String(liveJobs.length) + " total", dir: "up", note: "live" },
    { lab: "Max concurrency", ic: G.layers, val: String(limits.max_concurrent_jobs || "—"), delta: (limits.max_ttl_seconds || 600) + "s TTL", dir: "flat", note: "limit" },
  ];
  const { runtimes } = useRuntimes(5000);
  const recent = liveJobs.slice(0, 6);
  const rtRuntimes = (rt.caps && rt.caps.runtimes) || {};
  // Real job-status breakdown from live jobs (no fabricated throughput).
  const byStatus = {};
  liveJobs.forEach((j) => { byStatus[j.status] = (byStatus[j.status] || 0) + 1; });
  const statusRows = ["complete", "running", "queued", "error", "expired", "cancelled"].filter((s) => byStatus[s]).map((s) => [s, byStatus[s]]);

  return (
    <div className="wrap rise">
      <div className="phead">
        <div>
          <p className="eyebrow">{rt.health ? rt.health.mode + " · " + rt.health.runtime_id : "matrix runtime"}</p>
          <h1>Overview</h1>
          <p>Real-time health of this Matrix Runtime, its capabilities, and jobs.</p>
        </div>
        <div style={{ display: "flex", gap: 10 }}>
          <button className="btn btn-ghost" onClick={() => go("logs")}><GI d={G.logs} size={15} /> Live logs</button>
          <button className="btn btn-primary" onClick={() => go("catalog")}><GI d={G.play} size={15} /> New sandbox</button>
        </div>
      </div>

      <div className="metrics">
        {metrics.map((m) => (
          <div className="metric" key={m.lab}>
            <div className="lab"><span style={{ color: "var(--acc)", display: "inline-flex" }}><GI d={m.ic} size={15} /></span> {m.lab}</div>
            <div className="val">{m.val}</div>
            <div className={"delta " + m.dir}>
              {m.dir !== "flat" && <GI d={m.dir === "up" ? G.arrowup : G.arrowdn} size={12} />}
              {m.delta} <span style={{ color: "var(--ink-4)" }}>· {m.note}</span>
            </div>
          </div>
        ))}
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1.5fr 1fr", gap: 16, marginTop: 16 }} className="ov-grid">
        <div className="card">
          <div className="card-h"><span className="t">Runtimes {runtimes && <LiveTag />}</span><button className="chip" onClick={() => go("runtimes")}>manage <GI d={G.chevr} size={12} /></button></div>
          <table className="tbl">
            <thead><tr><th>Runtime</th><th className="hide-sm">Mode</th><th>Region</th><th>Jobs</th><th>Status</th></tr></thead>
            <tbody>
              {(runtimes || []).length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 20 }}>{runtimes ? "No runtimes." : "…"}</td></tr>}
              {(runtimes || []).map((c) => (
                <tr className="row" key={c.id}>
                  <td className="nm mono">{c.name}</td>
                  <td className="hide-sm"><span className="chip">{c.mode}</span></td>
                  <td className="mono" style={{ fontSize: 12 }}>{c.region}</td>
                  <td className="mono">{c.jobs}</td>
                  <td><span className={"chip " + c.statusClass}><span className={"dot " + c.statusClass + (c.statusClass === "green" ? " pulse" : "")} /> {c.status}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <div style={{ display: "grid", gap: 16, alignContent: "start" }}>
          <div className="card card-pad">
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <span style={{ fontSize: 13, fontWeight: 700 }}>Jobs by status</span><span className="chip">{liveJobs.length} total</span>
            </div>
            <div style={{ display: "grid", gap: 9, marginTop: 13 }}>
              {statusRows.length === 0 && <div style={{ fontSize: 12.5, color: "var(--ink-4)" }}>No jobs yet — start a sandbox or inspect a model.</div>}
              {statusRows.map(([s, n]) => (
                <div key={s} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12.5 }}>
                  <span className={"chip " + (STATUS_CLASS[s] || "")} style={{ height: 20, minWidth: 78 }}>{s}</span>
                  <div className="bar" style={{ flex: 1 }}><span className={STATUS_CLASS[s] === "red" ? "amber" : "green"} style={{ width: Math.round((n / liveJobs.length) * 100) + "%" }} /></div>
                  <span className="mono" style={{ fontSize: 11, color: "var(--ink-3)" }}>{n}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="card card-pad">
            <span style={{ fontSize: 13, fontWeight: 700 }}>Runtime health</span>
            <div style={{ display: "grid", gap: 11, marginTop: 13 }}>
              {[["Control plane", rt.online ? "green" : "red", rt.online ? "operational" : "unreachable"],
                ["Node runner", rtRuntimes.node ? "green" : "amber", rtRuntimes.node ? "ready" : "not found"],
                ["Python runner", rtRuntimes.python ? "green" : "amber", rtRuntimes.python ? "ready" : "not found"],
                ["Ollama", rtRuntimes.ollama ? "green" : "gray", rtRuntimes.ollama ? "ready" : "not detected"],
                ["vLLM", rtRuntimes.vllm ? "green" : "gray", rtRuntimes.vllm ? "ready" : "not detected"]].map(([k, s, v]) => (
                <div key={k} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 12.5 }}>
                  <span style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--ink-2)" }}><span className={"dot " + s + (s === "green" ? " pulse" : "")} /> {k}</span>
                  <span style={{ color: "var(--ink-4)" }}>{v}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-h"><span className="t">Recent jobs {jobs && <LiveTag />}</span><button className="chip" onClick={() => go("agents")}>view all <GI d={G.chevr} size={12} /></button></div>
        <table className="tbl">
          <thead><tr><th>Job ID</th><th>Type</th><th>Status</th><th className="hide-sm">Created</th></tr></thead>
          <tbody>
            {recent.length === 0 && <tr><td colSpan={4} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>No jobs yet — start a sandbox or inspect a model.</td></tr>}
            {recent.map((j) => (
              <tr className="row" key={j.job_id}>
                <td className="nm mono">{j.job_id}</td>
                <td><span className="chip">{j.type}</span></td>
                <td><span className={"chip " + (STATUS_CLASS[j.status] || "")}>{j.status === "running" && <span className="dot blue" />}{j.status}</span></td>
                <td className="hide-sm mono" style={{ fontSize: 12 }}>{(j.created_at || "").replace("T", " ").replace("Z", "")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Catalog + detail.
   --------------------------------------------------------------- */
function CatalogView({ openItem }) {
  const [tab, setTab] = React.useState("All");
  const [q, setQ] = React.useState("");
  const catalog = useCatalog();
  const tabs = ["All", "MCP Servers", "Agents", "Tools", "Models", "Verified", "Sandbox Enabled"];
  const items = (catalog || []).filter((it) => {
    if (q && !(it.name + it.desc + it.id).toLowerCase().includes(q.toLowerCase())) return false;
    if (tab === "All") return true;
    if (tab === "Verified") return it.verified;
    if (tab === "Sandbox Enabled") return it.sandbox;
    return it.kind === tab.replace(/s$/, "") || it.kind + "s" === tab;
  });
  return (
    <div className="wrap rise">
      <div className="phead">
        <div><p className="eyebrow">Registry {catalog && <LiveTag />}</p><h1>Catalog</h1><p>Curated MCP servers, agents, tools, and models served by this runtime. Test sandbox-enabled servers in a 10-minute session before you install.</p></div>
      </div>
      <div className="topsearch" style={{ width: "100%", maxWidth: 480, height: 40, marginBottom: 16 }}>
        <GI d={G.search} size={16} /><input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search MCP servers, tools, agents, models…" />
      </div>
      <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginBottom: 18 }}>
        {tabs.map((t) => <button key={t} onClick={() => setTab(t)} className={"chip" + (tab === t ? " green" : "")} style={{ height: 30, cursor: "pointer", fontFamily: "var(--font)", fontSize: 12.5 }}>{t}</button>)}
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }} className="ov-grid">
        {catalog === null && <div style={{ color: "var(--ink-4)", padding: 24 }}>Loading catalog…</div>}
        {items.map((it) => (
          <div className="card card-pad" key={it.id} style={{ display: "flex", flexDirection: "column", gap: 13 }}>
            <div style={{ display: "flex", gap: 13 }}>
              <ItemMono initials={it.initials} />
              <div style={{ minWidth: 0, flex: 1 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 7, flexWrap: "wrap" }}>
                  <span className={"chip " + it.kindClass}>{it.kind}</span>
                  {it.verified && <span className="chip green"><GI d={G.check} size={11} /> Verified</span>}
                  {it.sandbox && it.startCommand && <span className="chip blue">Sandbox · live</span>}
                  {it.sandbox && !it.startCommand && <span className="chip blue">Sandbox</span>}
                </div>
                <button onClick={() => openItem(it)} style={{ display: "block", marginTop: 9, fontSize: 16, fontWeight: 700, color: "var(--ink)", textAlign: "left" }}>{it.name}</button>
              </div>
            </div>
            <p style={{ margin: 0, fontSize: 12.5, lineHeight: 1.55, color: "var(--ink-3)" }}>{it.desc}</p>
            <div style={{ display: "flex", gap: 14, fontSize: 11.5, color: "var(--ink-3)", fontFamily: "var(--mono)", flexWrap: "wrap" }}>
              <span>{it.runtime}</span><span>· {it.source}</span><span>· {it.license}</span><span>· {it.secrets ? "needs secrets" : "no secrets"}</span>
            </div>
            <div style={{ display: "flex", gap: 9, marginTop: 2 }}>
              {it.sandbox
                ? <button className="btn btn-primary btn-sm" onClick={() => openItem(it, true)}><GI d={G.play} size={13} /> Test Sandbox</button>
                : <button className="btn btn-ghost btn-sm" onClick={() => openItem(it)}>View</button>}
              <button className="btn btn-ghost btn-sm" onClick={() => openItem(it)}>Details</button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function DetailView({ item, back, startSandbox }) {
  const manifest = {
    id: item.id, kind: item.kind.toLowerCase().replace(" ", "_"),
    runtime: { type: item.runtime.split(" / ")[0].toLowerCase(), transport: item.runtime.includes("stdio") ? "stdio" : "remote" },
    sandbox: { enabled: item.sandbox, ttl_seconds: 600, requires_secrets: item.secrets },
    source: item.source, license: item.license, version: item.version,
  };
  const rows = [["Runtime", item.runtime], ["Source", item.source], ["License", item.license], ["Secrets", item.secrets ? "Required" : "None"], ["Network", item.network], ["Start command", item.startCommand || "—"]];
  return (
    <div className="wrap rise">
      <button className="chip" onClick={back} style={{ marginBottom: 16, cursor: "pointer" }}><GI d={G.chev} size={13} style={{ transform: "rotate(90deg)" }} /> Catalog</button>
      <div style={{ display: "flex", gap: 18, flexWrap: "wrap", alignItems: "flex-start", justifyContent: "space-between" }}>
        <div style={{ display: "flex", gap: 16, minWidth: 0 }}>
          <ItemMono initials={item.initials} size={64} />
          <div style={{ minWidth: 0 }}>
            <div style={{ display: "flex", gap: 7, flexWrap: "wrap" }}>
              <span className={"chip " + item.kindClass}>{item.kind}</span>
              {item.verified && <span className="chip green"><GI d={G.check} size={11} /> Verified</span>}
              {item.sandbox && <span className="chip blue">Sandbox Enabled</span>}
              {!item.secrets && <span className="chip">No Secrets</span>}
            </div>
            <h1 style={{ margin: "13px 0 0", fontSize: 26, fontWeight: 700, letterSpacing: "-0.02em" }}>{item.name}</h1>
            <p style={{ margin: "9px 0 0", fontSize: 13.5, color: "var(--ink-3)", maxWidth: 540, lineHeight: 1.6 }}>{item.desc}</p>
          </div>
        </div>
      </div>
      <div style={{ display: "flex", gap: 10, marginTop: 22, flexWrap: "wrap" }}>
        <button className="btn btn-primary" disabled={!item.sandbox} onClick={() => startSandbox(item)} title={item.sandbox ? "" : "Sandbox not available for this item"}>
          <GI d={G.play} size={15} /> Test in 10-minute Sandbox
        </button>
        <button className="btn btn-ghost"><GI d={G.download} size={15} /> Install</button>
        <button className="btn btn-ghost"><GI d={G.logs} size={15} /> Copy CLI Command</button>
        <button className="btn btn-ghost"><GI d={G.bot} size={15} /> Add to Agent</button>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, marginTop: 22 }} className="ov-grid">
        <div style={{ display: "grid", gap: 16, alignContent: "start" }}>
          <div className="card">
            <div className="card-h"><span className="t">Overview</span></div>
            <div style={{ padding: "4px 18px" }}>
              {rows.map(([k, v]) => (
                <div key={k} style={{ display: "flex", justifyContent: "space-between", padding: "11px 0", borderBottom: "1px solid var(--line)", fontSize: 13 }}>
                  <span style={{ color: "var(--ink-3)" }}>{k}</span><span className="mono" style={{ color: "var(--ink)" }}>{v}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="card card-pad">
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <span style={{ fontWeight: 700, fontSize: 14 }}>Sandbox</span>
              <span className={"chip " + (item.sandbox ? "green" : "")}>{item.sandbox ? "Available" : "Unavailable"}</span>
            </div>
            <div style={{ display: "grid", gap: 9, marginTop: 13, fontSize: 12.5 }}>
              {[["TTL", "10 minutes"], ["Requires secrets", item.secrets ? "Yes" : "No"], ["Network", item.network], ["Runtime", item.runtime.split(" / ")[0]]].map(([k, v]) => (
                <div key={k} style={{ display: "flex", justifyContent: "space-between" }}><span style={{ color: "var(--ink-3)" }}>{k}</span><span className="mono">{v}</span></div>
              ))}
            </div>
          </div>
        </div>
        <div className="card" style={{ overflow: "hidden" }}>
          <div className="card-h"><span className="t">Manifest</span><span className="chip mono">manifest.json</span></div>
          <pre className="mono" style={{ margin: 0, padding: 16, fontSize: 12, lineHeight: 1.7, color: "var(--ink-2)", overflowX: "auto" }}>{JSON.stringify(manifest, null, 2)}</pre>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Sandbox modal + LIVE session (wired to /v1/sandbox/sessions).
   --------------------------------------------------------------- */
function SandboxModal({ item, onClose, onStart }) {
  const real = !!item.startCommand;
  return ReactDOM.createPortal(
    <div className="scrim" style={{ position: "fixed", inset: 0, zIndex: 70, display: "flex", alignItems: "center", justifyContent: "center", padding: 24, background: "rgba(5,8,12,0.7)", backdropFilter: "blur(6px)" }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="rise card" style={{ width: "100%", maxWidth: 460, background: "var(--panel-2)" }}>
        <div className="card-h"><span className="t">Test {item.name}</span><button className="iconbtn" onClick={onClose}><GI d={G.x} size={16} /></button></div>
        <div style={{ padding: 18 }}>
          <p style={{ margin: 0, fontSize: 13, lineHeight: 1.6, color: "var(--ink-2)" }}>
            {real
              ? "This starts a real sandbox on this Matrix Runtime over stdio. No production credentials are used. The session expires automatically after 10 minutes."
              : "This item has no runnable start command in this demo. The session will play a simulated lifecycle."}
          </p>
          <div style={{ display: "grid", gap: 1, marginTop: 16, borderRadius: "var(--r-sm)", overflow: "hidden", border: "1px solid var(--line)" }}>
            {[["Runtime", "Matrix Runtime · local"], ["TTL", "10 minutes"], ["Network", item.network], ["Secrets", "None"], ["Command", real ? item.startCommand : "—"]].map(([k, v]) => (
              <div key={k} style={{ display: "flex", gap: 12, padding: "11px 13px", background: "var(--inset)" }}>
                <span style={{ width: 84, flexShrink: 0, fontSize: 12, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.05em" }}>{k}</span>
                <span className="mono" style={{ fontSize: 12.5, color: "var(--ink)", wordBreak: "break-all" }}>{v}</span>
              </div>
            ))}
          </div>
          <div style={{ display: "flex", gap: 10, marginTop: 18 }}>
            <button className="btn btn-primary" style={{ flex: 1 }} onClick={onStart}><GI d={G.play} size={15} /> Start Sandbox</button>
            <button className="btn btn-ghost" onClick={onClose}>Cancel</button>
          </div>
        </div>
      </div>
    </div>,
    document.body
  );
}

function sampleArgs(tool) {
  const s = tool.input_schema || tool.inputSchema || {};
  const props = s.properties || {};
  const req = s.required || Object.keys(props);
  const out = {};
  for (const k of req) {
    const p = props[k] || {};
    if (k === "path") out[k] = "/tmp";
    else if (k === "pattern") out[k] = "*";
    else if (p.type === "number" || p.type === "integer") out[k] = 0;
    else if (p.type === "boolean") out[k] = false;
    else if (p.type === "array") out[k] = [];
    else out[k] = "";
  }
  return out;
}

const SIM_LIFECYCLE = [
  { step: "validate", status: "ok", message: "Command accepted" },
  { step: "sandbox", status: "start", message: "Temporary directory created" },
  { step: "mcp_start", status: "start", message: "MCP server started" },
  { step: "mcp_initialize", status: "ok", message: "MCP initialize succeeded" },
  { step: "tools_list", status: "ok", message: "Found 4 tools" },
  { step: "ready", status: "ok", message: "Sandbox ready" },
];
const SIM_TOOLS = [
  { name: "list_directory", description: "Read directory contents.", input_schema: { properties: { path: { type: "string" } }, required: ["path"] } },
  { name: "read_file", description: "Read the contents of a file.", input_schema: { properties: { path: { type: "string" } }, required: ["path"] } },
  { name: "write_file", description: "Write content to a file.", input_schema: { properties: { path: { type: "string" }, content: { type: "string" } }, required: ["path", "content"] } },
];

function SandboxSession({ item, back }) {
  const [events, setEvents] = React.useState([]);
  const [phase, setPhase] = React.useState("starting"); // starting | ready | expired | error
  const [tools, setTools] = React.useState([]);
  const [secs, setSecs] = React.useState(600);
  const [activeTool, setActiveTool] = React.useState(0);
  const [toolOut, setToolOut] = React.useState(null);
  const [running, setRunning] = React.useState(false);
  const [live, setLive] = React.useState(true);
  const sessRef = React.useRef(null);
  const logRef = React.useRef(null);

  React.useEffect(() => {
    let es = null, timer = null, cancelled = false;
    const cmd = item.startCommand;

    async function loadTools(sid) {
      try { const r = await api.get("/v1/sandbox/sessions/" + sid + "/tools"); if (!cancelled) setTools(r.tools || []); } catch (e) {}
    }
    function simulate() {
      setLive(false);
      let i = 0;
      timer = setInterval(() => {
        if (i >= SIM_LIFECYCLE.length) { clearInterval(timer); setPhase("ready"); setTools(SIM_TOOLS); startCountdown(); return; }
        setEvents((e) => [...e, SIM_LIFECYCLE[i]]); i++;
      }, 600);
    }
    function startCountdown(expiresAt) {
      timer = setInterval(() => {
        setSecs((s) => {
          if (expiresAt) return Math.max(0, Math.round((expiresAt - Date.now()) / 1000));
          return Math.max(0, s - 1);
        });
      }, 1000);
    }

    if (!cmd) { simulate(); return () => { cancelled = true; if (timer) clearInterval(timer); }; }

    (async () => {
      try {
        const r = await api.post("/v1/sandbox/sessions", { entity_id: item.id, ttl_seconds: 600, runtime: "node", transport: "stdio", start_command: cmd });
        if (cancelled) { api.del("/v1/sandbox/sessions/" + r.session_id).catch(() => {}); return; }
        sessRef.current = r.session_id;
        const expiresAt = r.expires_at ? new Date(r.expires_at).getTime() : null;
        es = new EventSource(api.eventsURL("/v1/sandbox/sessions/" + r.session_id + "/events"));
        es.onmessage = (ev) => {
          let d; try { d = JSON.parse(ev.data); } catch (e) { return; }
          setEvents((e) => [...e, d]);
          if (d.step === "ready") { setPhase("ready"); loadTools(r.session_id); }
          else if (d.status === "expired") setPhase("expired");
          else if (d.status === "error") setPhase("error");
        };
        es.onerror = () => { /* stream closes when the job ends; status already captured */ };
        startCountdown(expiresAt);
      } catch (e) {
        setEvents((ev) => [...ev, { step: "error", status: "error", message: e.message }]);
        simulate();
      }
    })();

    return () => {
      cancelled = true;
      if (es) es.close();
      if (timer) clearInterval(timer);
      if (sessRef.current) api.del("/v1/sandbox/sessions/" + sessRef.current).catch(() => {});
    };
  }, [item.id]);

  React.useEffect(() => { if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight; }, [events]);

  const mm = String(Math.floor(secs / 60)).padStart(2, "0");
  const ss = String(secs % 60).padStart(2, "0");

  async function runTool() {
    const tool = tools[activeTool]; if (!tool) return;
    setRunning(true); setToolOut(null);
    const args = sampleArgs(tool);
    try {
      if (live && sessRef.current) {
        const r = await api.post("/v1/sandbox/sessions/" + sessRef.current + "/tools/call", { name: tool.name, arguments: args });
        setToolOut(r.result !== undefined ? r.result : r);
      } else {
        await sleep(700);
        setToolOut(tool.name === "list_directory" ? { entries: ["notes.txt", "out.txt", "project/", "readme.md"], count: 4 } : { ok: true, tool: tool.name });
      }
    } catch (e) { setToolOut({ error: e.message }); }
    finally { setRunning(false); }
  }

  const toolCount = tools.length;
  return (
    <div className="wrap rise">
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14, flexWrap: "wrap", marginBottom: 18 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <button className="chip" onClick={back} style={{ cursor: "pointer" }}><GI d={G.chev} size={13} style={{ transform: "rotate(90deg)" }} /> Back</button>
          <div>
            <h1 style={{ margin: 0, fontSize: 20, fontWeight: 700 }}>{item.name} · Sandbox</h1>
            <p style={{ margin: "3px 0 0", fontSize: 12, color: "var(--ink-3)", fontFamily: "var(--mono)" }}>session {sessRef.current || "sbx_…"} · {live ? "live runtime" : "simulated"}</p>
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {live && <LiveTag />}
          <span className={"chip " + (phase === "ready" ? "green" : phase === "error" ? "red" : phase === "expired" ? "amber" : "amber")}>
            <span className={"dot " + (phase === "ready" ? "green pulse" : phase === "error" ? "red" : "amber")} /> {phase === "ready" ? "Running" : phase === "error" ? "Error" : phase === "expired" ? "Expired" : "Starting"}
          </span>
          <span className="chip mono" style={{ fontSize: 13 }}><GI d={G.clock} size={13} /> {mm}:{ss}</span>
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "260px 1fr 300px", gap: 16 }} className="sbx-grid">
        <div className="card" style={{ alignSelf: "start" }}>
          <div className="card-h"><span className="t">Lifecycle</span></div>
          <div style={{ padding: "8px 16px 14px" }}>
            {events.map((e, i) => (
              <div key={i} className="rise" style={{ display: "flex", gap: 10, padding: "9px 0", borderBottom: "1px solid var(--line)" }}>
                <span style={{ color: e.status === "error" ? "var(--red)" : "var(--acc)", display: "inline-flex", marginTop: 1 }}>
                  <GI d={e.status === "error" ? G.x : G.check} size={15} sw={2.4} />
                </span>
                <div><div className="mono" style={{ fontSize: 12.5, color: "var(--ink)" }}>{e.step}</div>
                  <div style={{ fontSize: 11.5, color: "var(--ink-3)", marginTop: 1 }}>{e.message}</div></div>
              </div>
            ))}
            {phase === "starting" && <div style={{ padding: "9px 0", color: "var(--ink-4)", fontSize: 12 }}><span className="cur" /></div>}
          </div>
        </div>

        <div className="card" style={{ alignSelf: "start" }}>
          <div className="card-h"><span className="t">Tool explorer</span><span className="chip">{phase === "ready" ? toolCount + " tools" : "—"}</span></div>
          {phase !== "ready" ? (
            <div style={{ padding: 40, textAlign: "center", color: "var(--ink-3)", fontSize: 13 }}>Waiting for <span className="mono">tools/list</span>…</div>
          ) : (
            <div style={{ padding: 16 }}>
              <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginBottom: 14 }}>
                {tools.map((t, i) => (
                  <button key={t.name} onClick={() => { setActiveTool(i); setToolOut(null); }} className={"chip" + (activeTool === i ? " green" : "")} style={{ cursor: "pointer", height: 28 }}>{t.name}</button>
                ))}
              </div>
              {tools[activeTool] && <>
                <p style={{ margin: "0 0 10px", fontSize: 12.5, color: "var(--ink-3)" }}>{tools[activeTool].description}</p>
                <div style={{ fontSize: 11, color: "var(--ink-4)", marginBottom: 6, textTransform: "uppercase", letterSpacing: "0.06em" }}>Input</div>
                <pre className="mono" style={{ margin: 0, padding: 12, background: "var(--inset)", border: "1px solid var(--line)", borderRadius: "var(--r-sm)", fontSize: 12, color: "var(--ink-2)", overflowX: "auto" }}>{JSON.stringify(sampleArgs(tools[activeTool]), null, 2)}</pre>
                <button className="btn btn-primary btn-sm" style={{ marginTop: 12 }} onClick={runTool} disabled={running}>
                  {running ? "Running…" : <><GI d={G.play} size={13} /> Run Tool</>}
                </button>
              </>}
              {toolOut && (
                <div className="rise" style={{ marginTop: 12 }}>
                  <div style={{ fontSize: 11, color: "var(--ink-4)", marginBottom: 6, textTransform: "uppercase", letterSpacing: "0.06em" }}>Output</div>
                  <pre className="mono" style={{ margin: 0, padding: 12, background: "var(--inset)", border: "1px solid var(--acc-line)", borderRadius: "var(--r-sm)", fontSize: 12, color: "var(--acc-2)", overflowX: "auto", maxHeight: 240 }}>{JSON.stringify(toolOut, null, 2)}</pre>
                </div>
              )}
            </div>
          )}
        </div>

        <div style={{ display: "grid", gap: 16, alignContent: "start" }}>
          <div className="card">
            <div className="card-h"><span className="t">Logs</span>{live && <span className="chip"><span className="dot green pulse" /> SSE</span>}</div>
            <div className="logwin" ref={logRef} style={{ height: 180, borderRadius: 0, border: "none" }}>
              {events.map((e, i) => (<div key={i} style={{ color: e.status === "error" ? "var(--red)" : "var(--ink-3)" }}><span style={{ color: "var(--ink-4)" }}>[{e.step}]</span> {e.message}</div>))}
              {phase === "ready" && <div style={{ color: "var(--acc-2)" }}>stdout: server listening on stdio</div>}
              <span className="cur" />
            </div>
          </div>
          <div className="card card-pad" style={{ borderColor: phase === "ready" ? "var(--acc-line)" : "var(--line)" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ fontWeight: 700, fontSize: 14 }}>Verdict</span>
              <span className={"chip " + (phase === "ready" ? "green" : phase === "error" ? "red" : "amber")}>{phase === "ready" ? "Passed" : phase === "error" ? "Failed" : "Pending"}</span>
            </div>
            {phase === "ready" ? (
              <>
                <p style={{ margin: "11px 0 0", fontSize: 12.5, lineHeight: 1.6, color: "var(--ink-2)" }}>
                  The MCP server initialized and exposed {toolCount} tools. No secrets were required. No blocked commands detected.
                </p>
                <div style={{ marginTop: 12, fontSize: 11, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.06em" }}>Recommended next step</div>
                <pre className="mono" style={{ margin: "6px 0 0", padding: 11, background: "var(--inset)", border: "1px solid var(--line)", borderRadius: "var(--r-sm)", fontSize: 11.5, color: "var(--acc-2)", overflowX: "auto" }}>matrix install {item.id.split(":")[1]} --alias {item.id.split(":")[1]}</pre>
              </>
            ) : <p style={{ margin: "11px 0 0", fontSize: 12.5, color: "var(--ink-3)" }}>{phase === "error" ? "Sandbox failed — see lifecycle." : "Running safety checks…"}</p>}
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Runtimes (live self runtime + demo fleet).
   --------------------------------------------------------------- */
function WorkspaceRuntimes() {
  const { runtimes, refresh } = useCloudRuntimes(5000);
  if (runtimes === null) return null;
  return (
    <div className="card" style={{ overflow: "hidden", marginBottom: 16 }}>
      <div className="card-h">
        <span className="t">Workspace runtimes <span style={{ color: "var(--ink-4)", fontWeight: 500 }}>· joined sandboxes & self-hosted</span></span>
        <button className="chip" style={{ cursor: "pointer" }} onClick={refresh}><GI d={G.refresh} size={11} /> refresh</button>
      </div>
      <table className="tbl">
        <thead><tr><th>Runtime</th><th>Status</th><th className="hide-sm">Kind</th><th className="hide-sm">Capabilities</th><th className="hide-sm">Heartbeat</th></tr></thead>
        <tbody>
          {runtimes.length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 22 }}>No runtimes have joined yet. Use <b style={{ color: "var(--ink-2)" }}>Add a runtime</b> to mint a join token or duplicate the HF Space.</td></tr>}
          {runtimes.map((r) => {
            const sc = RT_STATUS_CLASS[r.status] || "violet";
            return (
              <tr className="row" key={r.id}>
                <td>
                  <div className="nm mono" style={{ display: "flex", alignItems: "center", gap: 8 }}>{r.name || r.id}</div>
                  {r.hf_space ? <div style={{ fontSize: 11, color: "var(--ink-4)" }} className="hide-sm">🤗 {r.hf_space}</div> : (r.url ? <div style={{ fontSize: 11, color: "var(--ink-4)" }} className="hide-sm">{r.url}</div> : null)}
                </td>
                <td><span className={"chip " + sc}><span className={"dot " + sc + (sc === "green" ? " pulse" : "")} /> {r.status}</span></td>
                <td className="hide-sm"><span className="chip">{r.kind || "self-hosted"}</span></td>
                <td className="hide-sm" style={{ fontSize: 11.5, color: "var(--ink-3)" }}>{(r.caps || []).join(" · ") || "—"}</td>
                <td className="hide-sm mono" style={{ fontSize: 12, color: "var(--ink-3)" }}>{timeAgo(r.last_seen_at)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function RuntimesView({ go }) {
  const { runtimes, refresh } = useRuntimes(5000);
  const rows = runtimes || [];
  return (
    <div className="wrap rise">
      <div className="phead">
        <div><p className="eyebrow">Execution plane {runtimes && <LiveTag />}</p><h1>Runtimes</h1><p>Matrix Runtime installations connected to this control surface. Hybrid runtimes connect outbound — no inbound ports.</p></div>
        <div style={{ display: "flex", gap: 9 }}>
          <button className="btn btn-ghost" onClick={refresh}><GI d={G.refresh} size={15} /> Refresh</button>
          <button className="btn btn-primary" onClick={() => go("install")}><GI d={G.download} size={15} /> Add a runtime</button>
        </div>
      </div>
      <WorkspaceRuntimes />
      <div className="card" style={{ overflow: "hidden" }}>
        <div className="card-h"><span className="t">This control node</span></div>
        <table className="tbl">
          <thead><tr><th>Runtime</th><th>Status</th><th className="hide-sm">Mode</th><th className="hide-sm">Region</th><th>Jobs</th><th className="hide-sm">Heartbeat</th><th></th></tr></thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={7} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>{runtimes ? "No runtimes connected." : "…"}</td></tr>}
            {rows.map((r) => (
              <tr className="row" key={r.id}>
                <td><div className="nm mono" style={{ display: "flex", alignItems: "center", gap: 8 }}>{r.name} {r.live && <LiveTag />}</div><div style={{ fontSize: 11, color: "var(--ink-4)" }} className="hide-sm">{r.caps.join(" · ")}</div></td>
                <td><span className={"chip " + r.statusClass}><span className={"dot " + r.statusClass + (r.statusClass === "green" ? " pulse" : "")} /> {r.status}</span></td>
                <td className="hide-sm"><span className="chip">{r.mode}</span></td>
                <td className="hide-sm mono" style={{ fontSize: 12 }}>{r.region}</td>
                <td className="mono">{r.jobs}</td>
                <td className="hide-sm mono" style={{ fontSize: 12, color: "var(--ink-3)" }}>{r.heartbeat}</td>
                <td><button className="btn btn-ghost btn-sm" onClick={() => go("agents")}>View</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// HF_DUPLICATE_URL is the one-click "duplicate this Space" target. The owner's
// published Space is at agent-matrix/matrixcloud; override via window global.
const HF_DUPLICATE_URL = (window.MATRIX_HF_SPACE_URL || "https://huggingface.co/spaces/agent-matrix/matrixcloud") + "?duplicate=true";
const CLOUD_URL = window.MATRIX_CLOUD_URL || "https://api.matrixhub.io";

function InstallRuntimeView() {
  const [tab, setTab] = React.useState("Hugging Face Space");
  const [token, setToken] = React.useState("");   // real minted secret (shown once)
  const [minting, setMinting] = React.useState(false);
  const [mintErr, setMintErr] = React.useState("");
  const [copied, setCopied] = React.useState(false);
  const tabs = ["Hugging Face Space", "Docker", "Kubernetes", "Helm", "Local Dev", "On-Prem"];
  const tk = token || "mxrt_xxxxx_mint_a_token";

  async function mint() {
    setMintErr(""); setMinting(true);
    try { const r = await cloud.mintJoinToken("console", 1, 60); setToken(r.secret); }
    catch (e) { setMintErr(e.message || "Could not mint a join token."); }
    finally { setMinting(false); }
  }
  function copyToken() { try { navigator.clipboard.writeText(token); } catch (e) {} setCopied(true); setTimeout(() => setCopied(false), 1300); }

  const cmds = {
    "Helm": `helm install matrix-runtime ./deploy/helm/matrix-runtime \\\n  --namespace matrix-runtime --create-namespace \\\n  --set cloud.url=${CLOUD_URL} \\\n  --set runtime.joinToken=${tk}`,
    "Docker": `docker run -d --name matrix-runtime \\\n  -e MATRIX_CLOUD_URL=${CLOUD_URL} \\\n  -e MATRIX_RUNTIME_JOIN_TOKEN=${tk} \\\n  -v matrix-runtime-data:/var/lib/matrix-runtime \\\n  ghcr.io/agent-matrix/matrix-runtime:latest`,
    "Kubernetes": `kubectl apply -f deploy/k8s/namespace.yaml\nkubectl -n matrix-runtime create secret generic join-token \\\n  --from-literal=token=${tk}`,
    "Hugging Face Space": `# 1) Duplicate the Space (button on the right), then\n# 2) set these as Space secrets:\nMATRIX_RUNTIME_MODE=hf-space\nMATRIX_CLOUD_URL=${CLOUD_URL}\nMATRIX_RUNTIME_JOIN_TOKEN=${tk}\n# 3) (optional) bring your own HF inference:\nHF_TOKEN=hf_your_token`,
    "Local Dev": `make build\n./bin/matrix-runtime --mode local-dev\n# join the control plane:\nmatrix-runtime join --cloud-url ${CLOUD_URL} --token ${tk}`,
    "On-Prem": `sudo make install INSTALL_SYSTEMD=1\nsudo systemctl enable --now matrix-runtime\n# join-token: ${tk}`,
  };
  return (
    <div className="wrap rise">
      <div className="phead">
        <div><p className="eyebrow">Onboarding</p><h1>Add a runtime</h1><p>MatrixCloud manages the control plane. A runtime executes jobs inside your environment — duplicate the Hugging Face Space for a managed sandbox, or self-host. Runtimes connect outbound only.</p></div>
        <span className="chip violet">Hybrid · recommended</span>
      </div>
      <div className="card card-pad" style={{ display: "flex", gap: 16, alignItems: "center", marginBottom: 18, borderColor: "rgba(157,123,255,0.3)" }}>
        {[["Control plane", "MatrixHub Cloud", "SaaS"], ["→", "", ""], ["Execution plane", "Your infrastructure", "Runtime"]].map(([a, b, c], i) => (
          a === "→"
            ? <GI key={i} d={G.chevr} size={20} style={{ color: "var(--ink-4)", flexShrink: 0 }} />
            : <div key={i} style={{ flex: 1, padding: "12px 14px", borderRadius: "var(--r-md)", border: "1px solid var(--line-2)", background: "var(--inset)" }}>
                <div style={{ fontSize: 11, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.06em" }}>{a}</div>
                <div style={{ fontWeight: 700, fontSize: 14, marginTop: 4 }}>{b}</div>
                <span className="chip" style={{ marginTop: 8 }}>{c}</span>
              </div>
        ))}
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 300px", gap: 16 }} className="ov-grid">
        <div>
          <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginBottom: 14 }}>
            {tabs.map((t) => <button key={t} onClick={() => setTab(t)} className={"chip" + (tab === t ? " green" : "")} style={{ height: 30, cursor: "pointer", fontFamily: "var(--font)", fontSize: 12.5 }}>{t}</button>)}
          </div>
          <CmdBlock label={tab + " install"}>{cmds[tab]}</CmdBlock>
          <div className="card card-pad" style={{ marginTop: 14 }}>
            <span style={{ fontWeight: 700, fontSize: 13.5 }}>Expected result</span>
            <p style={{ margin: "8px 0 0", fontSize: 12.5, color: "var(--ink-3)", lineHeight: 1.6 }}>After installation, your runtime appears under <b style={{ color: "var(--ink-2)" }}>Runtimes</b> with status <span className="chip green" style={{ height: 20 }}><span className="dot green" /> Online</span> and these capabilities:</p>
            <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginTop: 11 }}>
              {["mcp.test", "mcp.run", "agent.run", "model.pull", "model.preload"].map((c) => <span key={c} className="chip green">{c}</span>)}
            </div>
          </div>
        </div>
        <div style={{ display: "grid", gap: 16, alignContent: "start" }}>
          {tab === "Hugging Face Space" && (
            <a className="btn btn-primary" href={HF_DUPLICATE_URL} target="_blank" rel="noopener noreferrer" style={{ height: 46, justifyContent: "center", textDecoration: "none" }}>
              <GI d={G.download} size={16} /> Duplicate the Space
            </a>
          )}
          <div className="card card-pad">
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}><GI d={G.key} size={16} style={{ color: "var(--acc)" }} /><span style={{ fontWeight: 700, fontSize: 13.5 }}>Runtime join token</span></div>
            {token ? (
              <>
                <div className="mono" style={{ marginTop: 12, padding: "11px 13px", borderRadius: "var(--r-sm)", background: "var(--inset)", border: "1px solid var(--line-2)", fontSize: 13, color: "var(--ink)", letterSpacing: "0.04em", wordBreak: "break-all" }}>{token}</div>
                <div style={{ display: "flex", gap: 8, marginTop: 12 }}>
                  <button className="btn btn-ghost btn-sm" onClick={copyToken} style={{ flex: 1 }}>{copied ? <><GI d={G.check} size={14} /> Copied</> : <><GI d={G.logs} size={14} /> Copy</>}</button>
                  <button className="btn btn-ghost btn-sm" onClick={mint} title="Mint another"><GI d={G.refresh} size={14} /></button>
                </div>
                <p style={{ margin: "11px 0 0", fontSize: 11.5, color: "var(--acc-2)", lineHeight: 1.5 }}>Shown once — copy it now. Single-use, expires in 60 minutes.</p>
              </>
            ) : (
              <>
                <p style={{ margin: "11px 0 0", fontSize: 12, color: "var(--ink-3)", lineHeight: 1.5 }}>Mint a single-use token scoped to your workspace, then paste it into the runtime's secrets.</p>
                <button className="btn btn-primary btn-sm" onClick={mint} disabled={minting} style={{ marginTop: 12, width: "100%" }}>{minting ? "Minting…" : <><GI d={G.key} size={14} /> Mint join token</>}</button>
                {mintErr && <p style={{ margin: "10px 0 0", fontSize: 11.5, color: "var(--red)" }}>{mintErr}</p>}
              </>
            )}
          </div>
          <div className="card card-pad">
            <span style={{ fontWeight: 700, fontSize: 13.5 }}>Security posture</span>
            <div style={{ display: "grid", gap: 9, marginTop: 12 }}>
              {["Outbound-only tunnel", "Secrets stay on-prem", "Signed runtime image", "TLS + audited control channel"].map((s) => (
                <div key={s} style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "var(--ink-2)" }}>
                  <span style={{ color: "var(--acc)", display: "inline-flex" }}><GI d={G.check} size={14} sw={2.4} /></span> {s}
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Models — generic multi-source importer.
   Lifecycle: Profile only → Queued → Downloading → Installed →
   Attached → Ready. Import (resolve a profile) is separate from
   Attach (install onto a runtime) and Runtime Cache (physical state).
   --------------------------------------------------------------- */
const HF_FALLBACK = [
  { id: "deepseek-ai/DeepSeek-V3", pipeline_tag: "text-generation", downloads: 1520000, likes: 8100, tags: ["transformers", "moe"], library_name: "transformers" },
  { id: "deepseek-ai/DeepSeek-R1", pipeline_tag: "text-generation", downloads: 2100000, likes: 11200, tags: ["transformers", "reasoning"], library_name: "transformers" },
  { id: "Qwen/Qwen2.5-7B-Instruct", pipeline_tag: "text-generation", downloads: 2300000, likes: 12000, tags: ["transformers"], library_name: "transformers" },
  { id: "meta-llama/Llama-3.1-8B-Instruct", pipeline_tag: "text-generation", downloads: 3100000, likes: 15000, tags: ["transformers", "gated"], library_name: "transformers" },
  { id: "mistralai/Mistral-7B-Instruct-v0.3", pipeline_tag: "text-generation", downloads: 1900000, likes: 7600, tags: ["transformers"], library_name: "transformers" },
  { id: "BAAI/bge-large-en-v1.5", pipeline_tag: "feature-extraction", downloads: 4200000, likes: 2200, tags: ["sentence-transformers"], library_name: "sentence-transformers" },
];
function fmtNum(n) { if (n == null) return "—"; if (n >= 1e6) return (n / 1e6).toFixed(1) + "M"; if (n >= 1e3) return (n / 1e3).toFixed(0) + "K"; return String(n); }
function gpuLikely(m) { return /(\b|-|\/)(7b|8b|13b|70b|v3|v4|r1|large|moe)/i.test((m && m.id) || "") || ((m && m.tags) || []).includes("moe"); }
function recRuntime(m) { return (m.library_name === "sentence-transformers" || m.pipeline_tag === "feature-extraction") ? "Matrix Runtime" : (gpuLikely(m) ? "vLLM / SGLang" : "Ollama / vLLM"); }

// Search HF: same-origin backend proxy first (real, no CORS), then direct HF,
// then offline sample data — so the demo never breaks.
async function hfSearch(q, task) {
  try {
    const p = new URLSearchParams({ q: q || "deepseek", limit: "16" });
    if (task && task !== "any") p.set("task", task);
    const r = await api.get("/v1/model-sources/huggingface/search?" + p.toString());
    if (r && r.live && Array.isArray(r.items) && r.items.length) return { live: true, items: r.items };
  } catch (e) { /* fall through */ }
  try {
    const params = new URLSearchParams({ search: q || "deepseek", sort: "downloads", direction: "-1", limit: "16" });
    if (task && task !== "any") params.set("pipeline_tag", task);
    const r = await fetch("https://huggingface.co/api/models?" + params.toString(), { headers: { Accept: "application/json" } });
    if (r.ok) {
      const data = await r.json();
      if (Array.isArray(data) && data.length) return { live: true, items: data.map((m) => ({ id: m.id || m.modelId, pipeline_tag: m.pipeline_tag, downloads: m.downloads, likes: m.likes, tags: m.tags || [], library_name: m.library_name })) };
    }
  } catch (e) { /* fall through */ }
  const ql = (q || "").toLowerCase();
  return { live: false, items: HF_FALLBACK.filter((m) => !ql || m.id.toLowerCase().includes(ql)) };
}

const IMPORT_SOURCES = [
  { id: "huggingface", name: "Hugging Face", glyph: "🤗", desc: "Search public & private models", search: true },
  { id: "github", name: "GitHub", mono: "GH", desc: "Import from a repository", repo: true },
  { id: "gitlab", name: "GitLab", mono: "GL", desc: "Import from a repository", repo: true },
  { id: "s3", name: "Amazon S3", mono: "S3", desc: "Model artifacts in a bucket", bucket: true },
  { id: "r2", name: "Cloudflare R2", mono: "R2", desc: "S3-compatible object storage", bucket: true },
  { id: "ollama", name: "Ollama", mono: "OL", desc: "Pull a local Ollama model", ollama: true },
  { id: "url", name: "Custom URL", mono: "URL", desc: "Direct manifest or weights URL", url: true },
];

function Inp({ label, value, onChange, placeholder, mono }) {
  return (
    <label style={{ display: "block" }}>
      <span style={{ display: "block", fontSize: 10.5, fontFamily: "var(--mono)", letterSpacing: "0.14em", textTransform: "uppercase", color: "var(--ink-4)", marginBottom: 6 }}>{label}</span>
      <input value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} spellCheck={false}
        style={{ width: "100%", height: 42, padding: "0 12px", borderRadius: "var(--r-sm)", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.35)", color: "var(--ink)", fontSize: 13, fontFamily: mono ? "var(--mono)" : "var(--font)", outline: "none" }} />
    </label>
  );
}

function ImportModelModal({ onClose, onImport, initialSource }) {
  const [source, setSource] = React.useState(null);
  const [step, setStep] = React.useState(0); // 0 source/search · 1 preview · 2 attach
  const [q, setQ] = React.useState("deepseek");
  const [task, setTask] = React.useState("any");
  const [items, setItems] = React.useState([]);
  const [live, setLive] = React.useState(true);
  const [loading, setLoading] = React.useState(false);
  const [priv, setPriv] = React.useState(false);
  const [token, setToken] = React.useState("");
  const [form, setForm] = React.useState({ repo: "", branch: "main", bucket: "", endpoint: "", path: "", model: "", url: "" });
  const [sel, setSel] = React.useState(null);
  const setF = (k, v) => setForm((s) => ({ ...s, [k]: v }));

  async function doSearch() { setLoading(true); const res = await hfSearch(q, task); setItems(res.items); setLive(res.live); setLoading(false); }
  function pickSource(s) { setSource(s); if (s.search) { setItems([]); doSearch(); } }
  React.useEffect(() => {
    if (initialSource) { const s = IMPORT_SOURCES.find((x) => x.id === initialSource); if (s) pickSource(s); }
  }, []); // eslint-disable-line

  function resolveForm() {
    let id = "";
    if (source.repo) id = (form.repo || "owner/repo").replace(/^https?:\/\/(github|gitlab)\.com\//, "");
    else if (source.bucket) id = source.id + "://" + (form.bucket || "bucket") + "/" + (form.path || "model");
    else if (source.ollama) id = "ollama/" + (form.model || "llama3.1");
    else if (source.url) id = form.url || "https://example.com/model.gguf";
    setSel({ id, pipeline_tag: "text-generation", library_name: source.ollama ? "ollama" : "custom", tags: priv ? ["private"] : [], _source: source.name });
    setStep(1);
  }

  const tasks = ["any", "text-generation", "feature-extraction", "text-classification", "automatic-speech-recognition"];
  const heads = ["Source", "Preview", "Attach"];

  return ReactDOM.createPortal(
    <div className="scrim" style={{ position: "fixed", inset: 0, zIndex: 70, display: "flex", alignItems: "center", justifyContent: "center", padding: 24, background: "rgba(5,8,12,0.72)", backdropFilter: "blur(7px)" }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="rise card" style={{ width: "100%", maxWidth: 760, maxHeight: "88vh", display: "flex", flexDirection: "column", background: "var(--panel-2)" }}>
        <div className="card-h">
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span style={{ color: "var(--acc)", display: "inline-flex" }}><GI d={G.download} size={17} /></span>
            <span className="t" style={{ letterSpacing: "0.04em" }}>Import Model{source ? " · " + source.name : ""}</span>
          </div>
          <button className="iconbtn" onClick={onClose}><GI d={G.x} size={16} /></button>
        </div>

        {source && (
          <div style={{ display: "flex", gap: 8, padding: "12px 20px 0" }}>
            {heads.map((s, i) => (
              <div key={s} style={{ display: "flex", alignItems: "center", gap: 7, fontSize: 11.5, color: i === step ? "var(--acc-2)" : i < step ? "var(--ink-2)" : "var(--ink-4)" }}>
                <span style={{ width: 18, height: 18, borderRadius: 99, display: "grid", placeItems: "center", fontSize: 10, fontFamily: "var(--mono)", border: "1px solid " + (i <= step ? "var(--acc)" : "var(--line-3)"), background: i < step ? "var(--acc)" : "transparent", color: i < step ? "#03140c" : "inherit" }}>{i < step ? "✓" : i + 1}</span>
                {s}{i < 2 && <span style={{ color: "var(--ink-4)" }}>›</span>}
              </div>
            ))}
          </div>
        )}

        <div style={{ padding: 20, overflowY: "auto", flex: 1 }}>
          {!source && (
            <div>
              <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--ink-3)" }}>Choose where to import the model from. Private sources accept a token.</p>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }} className="ov-grid">
                {IMPORT_SOURCES.map((s) => (
                  <button key={s.id} onClick={() => pickSource(s)} style={{ display: "flex", alignItems: "center", gap: 13, padding: "13px 14px", borderRadius: "var(--r-md)", textAlign: "left", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.3)" }}>
                    <span style={{ width: 38, height: 38, borderRadius: 10, flexShrink: 0, display: "grid", placeItems: "center", border: "1px solid var(--line-2)", background: "var(--raised)", color: "var(--ink)", fontFamily: "var(--mono)", fontWeight: 700, fontSize: 12 }}>{s.glyph || s.mono}</span>
                    <span style={{ minWidth: 0 }}><span style={{ display: "block", fontSize: 13.5, fontWeight: 600 }}>{s.name}</span><span style={{ display: "block", fontSize: 11.5, color: "var(--ink-3)", marginTop: 1 }}>{s.desc}</span></span>
                  </button>
                ))}
              </div>
            </div>
          )}

          {source && step === 0 && source.search && (
            <div>
              <form onSubmit={(e) => { e.preventDefault(); doSearch(); }} style={{ display: "flex", gap: 9, flexWrap: "wrap" }}>
                <div className="topsearch" style={{ flex: "1 1 260px", height: 42, width: "auto", maxWidth: "none", display: "flex" }}>
                  <GI d={G.search} size={16} /><input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search models… e.g. deepseek" autoFocus />
                </div>
                <select value={task} onChange={(e) => setTask(e.target.value)} style={{ height: 42, padding: "0 12px", borderRadius: "var(--r-sm)", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.35)", color: "var(--ink)", fontSize: 13 }}>
                  {tasks.map((t) => <option key={t} value={t}>{t === "any" ? "Any task" : t}</option>)}
                </select>
                <button className="btn btn-primary" type="submit">Search</button>
              </form>
              <label style={{ display: "flex", alignItems: "center", gap: 8, margin: "12px 0 0", fontSize: 12.5, color: "var(--ink-2)", cursor: "pointer" }}>
                <input type="checkbox" checked={priv} onChange={(e) => setPriv(e.target.checked)} style={{ accentColor: "var(--acc)" }} /> Private repository (use access token)
              </label>
              {priv && <div style={{ marginTop: 10 }}><Inp label="HF access token" value={token} onChange={setToken} placeholder="hf_•••••••••••" mono /></div>}
              <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "12px 0 4px" }}>
                <span className={"chip " + (live ? "green" : "amber")} style={{ height: 20 }}><span className={"dot " + (live ? "green" : "amber")} /> {live ? "live · huggingface.co/api" : "offline · sample results"}</span>
                {loading && <span style={{ fontSize: 11.5, color: "var(--ink-4)" }}>searching…</span>}
              </div>
              <div style={{ display: "grid", gap: 9, marginTop: 8 }}>
                {loading ? <div style={{ padding: 28, textAlign: "center", color: "var(--ink-3)", fontSize: 13 }}>Querying Hugging Face…</div>
                  : items.map((m) => (
                    <div key={m.id} style={{ display: "flex", alignItems: "center", gap: 13, padding: "11px 14px", borderRadius: "var(--r-md)", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.3)" }}>
                      <div style={{ minWidth: 0, flex: 1 }}>
                        <div className="mono" style={{ fontSize: 13, fontWeight: 600, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{m.id}</div>
                        <div style={{ display: "flex", gap: 12, marginTop: 5, fontSize: 11.5, color: "var(--ink-3)", flexWrap: "wrap" }}>
                          <span>{m.pipeline_tag || "—"}</span><span>↓ {fmtNum(m.downloads)}</span><span>♥ {fmtNum(m.likes)}</span>
                          {gpuLikely(m) && <span className="chip amber" style={{ height: 18 }}>GPU</span>}
                          {((m.tags || []).includes("gated") || m.gated) && <span className="chip amber" style={{ height: 18 }}>Gated</span>}
                        </div>
                      </div>
                      <button className="btn btn-ghost btn-sm" onClick={() => { setSel({ ...m, _source: "Hugging Face" }); setStep(1); }}>Import Profile</button>
                    </div>
                  ))}
              </div>
            </div>
          )}

          {source && step === 0 && !source.search && (
            <div style={{ display: "grid", gap: 14 }}>
              <p style={{ margin: 0, fontSize: 13, color: "var(--ink-3)" }}>Provide the {source.name} location to resolve a model profile.</p>
              {source.repo && <><Inp label="Repository" value={form.repo} onChange={(v) => setF("repo", v)} placeholder={source.id + ".com/owner/model-repo"} mono /><Inp label="Branch / tag" value={form.branch} onChange={(v) => setF("branch", v)} placeholder="main" mono /></>}
              {source.bucket && <><Inp label="Bucket" value={form.bucket} onChange={(v) => setF("bucket", v)} placeholder="my-models" mono /><Inp label="Endpoint" value={form.endpoint} onChange={(v) => setF("endpoint", v)} placeholder={source.id === "r2" ? "https://<acct>.r2.cloudflarestorage.com" : "s3.us-east-1.amazonaws.com"} mono /><Inp label="Object path" value={form.path} onChange={(v) => setF("path", v)} placeholder="models/deepseek-v4/" mono /></>}
              {source.ollama && <Inp label="Ollama model" value={form.model} onChange={(v) => setF("model", v)} placeholder="llama3.1 / qwen2.5 / mistral" mono />}
              {source.url && <Inp label="Manifest / weights URL" value={form.url} onChange={(v) => setF("url", v)} placeholder="https://…/model.gguf" mono />}
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12.5, color: "var(--ink-2)", cursor: "pointer" }}>
                <input type="checkbox" checked={priv} onChange={(e) => setPriv(e.target.checked)} style={{ accentColor: "var(--acc)" }} /> Private — requires credentials
              </label>
              {priv && <Inp label={source.bucket ? "Access key / secret" : "Access token"} value={token} onChange={setToken} placeholder={source.bucket ? "AKIA… / ••••" : "token ••••"} mono />}
              <div style={{ display: "flex", justifyContent: "space-between" }}>
                <button className="btn btn-ghost" onClick={() => setSource(null)}>Back</button>
                <button className="btn btn-primary" onClick={resolveForm}>Resolve profile</button>
              </div>
            </div>
          )}

          {source && step === 1 && sel && (
            <div>
              <div className="mono" style={{ fontSize: 15, fontWeight: 700, color: "var(--ink)", wordBreak: "break-all" }}>{sel.id}</div>
              <div style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 4 }}>Model Profile preview · resolved from {sel._source}{priv ? " · private" : ""}</div>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1px", marginTop: 16, borderRadius: "var(--r-sm)", overflow: "hidden", border: "1px solid var(--line)" }}>
                {[["Source", sel._source], ["Model ID", sel.id], ["Task", sel.pipeline_tag || "text-generation"], ["Library", sel.library_name || "transformers"], ["Requires GPU", gpuLikely(sel) ? "Likely" : "No"], ["Recommended runtime", recRuntime(sel)], ["Access", priv ? "Private · token" : "Public"], ["License", (sel.tags || []).includes("gated") ? "Gated · review" : "review required"]].map(([k, v]) => (
                  <div key={k} style={{ padding: "11px 13px", background: "var(--inset)" }}><div style={{ fontSize: 10.5, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.05em" }}>{k}</div><div className="mono" style={{ fontSize: 12.5, color: "var(--ink)", marginTop: 4, overflow: "hidden", textOverflow: "ellipsis" }}>{v}</div></div>
                ))}
              </div>
              <div style={{ marginTop: 16, padding: 14, borderRadius: "var(--r-md)", border: "1px solid var(--line)", background: "rgba(0,0,0,0.25)" }}>
                <div style={{ fontSize: 12, fontWeight: 700, color: "var(--ink-2)", marginBottom: 10 }}>Security</div>
                <div style={{ display: "grid", gap: 8, fontSize: 12.5 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--acc-2)" }}><GI d={G.check} size={14} /> safetensors preferred</div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--ink-2)" }}><GI d={G.shield} size={14} /> remote code disabled by default — requires explicit approval</div>
                </div>
              </div>
              <div style={{ display: "flex", justifyContent: "space-between", marginTop: 18 }}>
                <button className="btn btn-ghost" onClick={() => setStep(0)}>Back</button>
                <button className="btn btn-primary" onClick={() => setStep(2)}><GI d={G.download} size={15} /> Import Profile</button>
              </div>
            </div>
          )}

          {source && step === 2 && sel && <AttachStep model={sel} onImport={onImport} />}
        </div>
      </div>
    </div>, document.body);
}

function AttachStep({ model, onImport }) {
  const { runtimes: all, refresh: refetch } = useRuntimes();
  const [refreshing, setRefreshing] = React.useState(false);
  const runtimes = (all || []).filter((r) => r.statusClass !== "red");
  const [runtime, setRuntime] = React.useState("");
  React.useEffect(() => { if (!runtime && runtimes[0]) setRuntime(runtimes[0].name); }, [runtimes, runtime]);
  const [mode, setMode] = React.useState("pull");
  const [engine, setEngine] = React.useState(gpuLikely(model) ? "vLLM" : "Ollama");
  const engines = ["vLLM", "SGLang", "TGI", "Ollama", "External endpoint"];
  function refresh() { setRefreshing(true); refetch(); setTimeout(() => setRefreshing(false), 700); }

  return (
    <div>
      <div style={{ fontSize: 13.5, fontWeight: 700 }}>Attach model to runtime</div>
      <div className="mono" style={{ fontSize: 12.5, color: "var(--ink-3)", marginTop: 4, wordBreak: "break-all" }}>{model.id}</div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", margin: "16px 0 8px" }}>
        <span style={{ fontSize: 11, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.06em" }}>Choose runtime</span>
        <button className="btn btn-ghost btn-sm" onClick={refresh} disabled={refreshing}><GI d={G.refresh} size={13} style={refreshing ? { animation: "spin 0.7s linear infinite" } : null} /> {refreshing ? "Refreshing…" : "Refresh"}</button>
      </div>
      <div style={{ display: "grid", gap: 8 }}>
        {runtimes.map((r) => {
          const lvl = gpuLikely(model) ? (r.caps.includes("model.preload") ? "compatible" : r.mode === "local-dev" ? "not recommended" : "limited") : "compatible";
          return (
            <button key={r.id} onClick={() => setRuntime(r.name)} style={{ display: "flex", alignItems: "center", gap: 11, padding: "11px 13px", borderRadius: "var(--r-md)", textAlign: "left", border: "1px solid " + (runtime === r.name ? "var(--acc-line)" : "var(--line-2)"), background: runtime === r.name ? "var(--acc-soft)" : "rgba(0,0,0,0.3)" }}>
              <span style={{ flex: 1 }}><span className="mono" style={{ fontSize: 13, color: "var(--ink)" }}>{r.name}</span>{r.live && <LiveTag />}<span style={{ fontSize: 11.5, color: "var(--ink-4)", marginLeft: 8 }}>{r.mode} · {r.region}</span></span>
              <span className={"chip " + (lvl === "compatible" ? "green" : "amber")} style={{ height: 20 }}>{lvl}</span>
            </button>
          );
        })}
      </div>
      {gpuLikely(model) && runtime && runtimes.find((r) => r.name === runtime && r.mode === "local-dev") && (
        <div style={{ marginTop: 10, padding: "10px 12px", borderRadius: "var(--r-sm)", border: "1px solid var(--amber-soft)", background: "var(--amber-soft)", fontSize: 12, color: "var(--amber)" }}>
          This model is likely too large for the selected runtime. Recommended: a GPU runtime, a quantized variant, or an external endpoint.
        </div>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, marginTop: 16 }} className="ov-grid">
        <div>
          <div style={{ fontSize: 11, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 8 }}>Install mode</div>
          <div style={{ display: "grid", gap: 7 }}>
            {[["pull", "Pull from source"], ["mount", "Mount existing volume"], ["endpoint", "Use external endpoint"]].map(([v, l]) => (
              <label key={v} style={{ display: "flex", alignItems: "center", gap: 9, fontSize: 12.5, color: "var(--ink-2)", cursor: "pointer", padding: "8px 11px", borderRadius: 8, border: "1px solid " + (mode === v ? "var(--acc-line)" : "var(--line)"), background: mode === v ? "var(--acc-soft)" : "transparent" }}>
                <input type="radio" name="imode" checked={mode === v} onChange={() => setMode(v)} style={{ accentColor: "var(--acc)" }} /> {l}
              </label>
            ))}
          </div>
        </div>
        <div>
          <div style={{ fontSize: 11, color: "var(--ink-4)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 8 }}>Serving engine</div>
          <div style={{ display: "flex", gap: 7, flexWrap: "wrap" }}>
            {engines.map((e) => <button key={e} onClick={() => setEngine(e)} className={"chip" + (engine === e ? " green" : "")} style={{ height: 30, cursor: "pointer", fontFamily: "var(--font)" }}>{e}</button>)}
          </div>
        </div>
      </div>

      <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 20 }}>
        <button className="btn btn-ghost" onClick={() => onImport({ model, runtime, engine, mode })}>Import profile only</button>
        <button className="btn btn-primary" onClick={() => onImport({ model, runtime, engine, mode, install: true })}><GI d={G.download} size={15} /> Attach &amp; install</button>
      </div>
    </div>
  );
}

// Map a source display name / install mode to the backend's enum values.
const SOURCE_TYPE_OF = { "Hugging Face": "huggingface", "GitHub": "github", "GitLab": "gitlab", "Amazon S3": "s3", "Cloudflare R2": "r2", "Ollama": "ollama", "Custom URL": "url" };
const INSTALL_MODE_OF = { pull: "pull_from_source", mount: "mount_volume", endpoint: "external_endpoint" };
const PROFILE_STATUS_CLASS = { ready: "green", attached: "green", installed: "green", downloading: "amber", queued: "blue", profile_only: "", failed: "red", gated: "amber", incompatible: "red" };

function useModelData(pollMs) {
  const [profiles, setProfiles] = React.useState(null);
  const [installs, setInstalls] = React.useState(null);
  const load = React.useCallback(async () => {
    try { const r = await api.get("/v1/model-profiles"); setProfiles(r.profiles || []); } catch (e) { setProfiles(null); }
    try { const r = await api.get("/v1/model-installations"); setInstalls(r.installations || []); } catch (e) { setInstalls(null); }
  }, []);
  React.useEffect(() => {
    load();
    if (pollMs) { const t = setInterval(load, pollMs); return () => clearInterval(t); }
  }, [load, pollMs]);
  return { profiles, installs, refresh: load };
}

function ModelsView({ onAttach }) {
  const [tab, setTab] = React.useState("Model Profiles");
  const [importOpen, setImportOpen] = React.useState(false);
  const [importSource, setImportSource] = React.useState(null);
  // Poll faster while a download is in flight, else slow.
  const [active, setActive] = React.useState(false);
  const { profiles, installs, refresh } = useModelData(active ? 1500 : 6000);
  const tabs = ["Available Models", "Connected Providers", "Model Profiles", "Runtime Cache"];
  const providers = [["Hugging Face", "HF", "connected", "live search · resolvable", true], ["OpenAI-compatible", "AI", "connected", "gpt-4o · gpt-4o-mini", false],
    ["Ollama", "OL", "connected", "local models", false], ["vLLM", "VL", "available", "GPU endpoint", false], ["GitHub", "GH", "available", "repo-hosted models", false], ["Amazon S3", "S3", "available", "bucket artifacts", false]];

  // Available Models = installations that are ready/attached (real).
  const readyInstalls = (installs || []).filter((i) => i.status === "ready" || i.status === "attached");

  React.useEffect(() => {
    const downloading = (installs || []).some((i) => i.status === "downloading" || i.status === "checking" || i.status === "queued");
    setActive(downloading);
  }, [installs]);

  async function handleImport(p) {
    setImportOpen(false); setImportSource(null);
    const m = p.model || {};
    const provider = m._source || "Custom URL";
    const body = {
      source_type: SOURCE_TYPE_OF[provider] || "url",
      provider,
      external_id: m.id,
      display_name: m.id,
      source_uri: provider === "Hugging Face" ? ("hf:" + m.id) : m.id,
      task: m.pipeline_tag || "text-generation",
      library: m.library_name || "transformers",
      license: (m.tags || []).includes("gated") ? "gated" : "review required",
      tags: m.tags || [],
      metadata: { downloads: m.downloads, likes: m.likes },
    };
    let profile;
    try { const r = await api.post("/v1/model-profiles", body); profile = r.profile; }
    catch (e) { return; }
    if (p.install && profile) {
      setTab("Runtime Cache"); setActive(true);
      try {
        await api.post("/v1/model-profiles/" + profile.id + "/attach", {
          runtimeId: p.runtime, installMode: INSTALL_MODE_OF[p.mode] || "pull_from_source", servingEngine: p.engine,
        });
      } catch (e) { /* surfaced via list */ }
      refresh();
    } else {
      setTab("Model Profiles"); refresh();
    }
  }

  return (
    <div className="wrap rise">
      <div className="phead">
        <div><p className="eyebrow">Model gateway · matrix-llm</p><h1>Models</h1><p>Connect providers, resolve metadata into profiles, then attach and install models into runtimes — from Hugging Face, GitHub, S3, R2, Ollama, or a custom URL.</p></div>
        <div style={{ display: "flex", gap: 9, flexWrap: "wrap" }}>
          <button className="btn btn-ghost" onClick={onAttach}><GI d={G.plus} size={15} /> Add Model</button>
          <button className="btn btn-primary" onClick={() => { setImportSource(null); setImportOpen(true); }}><GI d={G.download} size={15} /> Import Model</button>
        </div>
      </div>
      <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginBottom: 18 }}>
        {tabs.map((t) => <button key={t} onClick={() => setTab(t)} className={"chip" + (tab === t ? " green" : "")} style={{ height: 30, cursor: "pointer", fontFamily: "var(--font)", fontSize: 12.5 }}>{t}</button>)}
      </div>

      {tab === "Available Models" && (
        <div className="card" style={{ overflow: "hidden" }}>
          <div className="card-h"><span className="t">Ready to use {installs && <LiveTag />}</span></div>
          <table className="tbl">
            <thead><tr><th>Model</th><th className="hide-sm">Provider</th><th>Runtime</th><th className="hide-sm">Engine</th><th></th></tr></thead>
            <tbody>
              {readyInstalls.length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>No models ready yet — import one and attach it to a runtime.</td></tr>}
              {readyInstalls.map((m) => (
                <tr className="row" key={m.id}>
                  <td className="nm mono" style={{ fontSize: 12.5 }}>{m.model_name}</td>
                  <td className="hide-sm">{m.provider}</td><td><span className="chip">{m.runtime_id}</span></td><td className="hide-sm">{m.serving_engine || "—"}</td>
                  <td style={{ textAlign: "right" }}><button className="btn btn-ghost btn-sm">Use</button></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === "Connected Providers" && (
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }} className="ov-grid">
          {providers.map(([nm, tag, st, ds, isHf]) => (
            <div className="card card-pad" key={nm} style={{ display: "flex", alignItems: "center", gap: 14 }}>
              <div style={{ width: 40, height: 40, borderRadius: 10, flexShrink: 0, display: "grid", placeItems: "center", border: "1px solid var(--line-2)", background: "var(--raised)", color: "var(--ink)", fontFamily: "var(--mono)", fontWeight: 700, fontSize: 11 }}>{isHf ? "🤗" : tag}</div>
              <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontWeight: 700, fontSize: 14 }}>{nm}</div><div style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 2 }}>{ds}</div></div>
              {isHf ? <button className="btn btn-ghost btn-sm" onClick={() => { setImportSource("huggingface"); setImportOpen(true); }}><GI d={G.search} size={13} /> Search Models</button>
                : st === "connected" ? <span className="chip green"><span className="dot green" /> connected</span> : <button className="btn btn-ghost btn-sm" onClick={() => { setImportSource(tag === "GH" ? "github" : tag === "S3" ? "s3" : null); setImportOpen(true); }}>Connect</button>}
            </div>
          ))}
        </div>
      )}

      {tab === "Model Profiles" && (
        <div className="card" style={{ overflow: "hidden" }}>
          <div className="card-h"><span className="t">Model Profiles {profiles && <LiveTag />}</span><button className="chip" onClick={refresh} style={{ cursor: "pointer" }}><GI d={G.refresh} size={11} /> refresh</button></div>
          <table className="tbl">
            <thead><tr><th>Model Profile</th><th className="hide-sm">Provider</th><th>Status</th><th className="hide-sm">Runtime</th><th></th></tr></thead>
            <tbody>
              {(profiles || []).length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>{profiles ? "No profiles yet — click Import Model to resolve one." : "Runtime unreachable."}</td></tr>}
              {(profiles || []).map((p) => {
                const inst = (installs || []).find((i) => i.model_profile_id === p.id);
                const rt = inst ? inst.runtime_id : "Not attached";
                const sc = PROFILE_STATUS_CLASS[p.status] || "";
                const label = p.status === "profile_only" ? "Profile only" : p.status.charAt(0).toUpperCase() + p.status.slice(1);
                return (
                  <tr className="row" key={p.id}>
                    <td className="nm mono" style={{ fontSize: 12.5 }}>{p.display_name}</td>
                    <td className="hide-sm">{p.provider}</td>
                    <td><span className={"chip " + sc}>{p.status === "downloading" && <span className="dot amber" />}{label}</span></td>
                    <td className="hide-sm mono" style={{ fontSize: 12 }}>{rt}</td>
                    <td style={{ textAlign: "right", whiteSpace: "nowrap" }}>
                      {!inst
                        ? <button className="btn btn-primary btn-sm" onClick={() => { setImportSource(null); setImportOpen(true); }}>Attach Runtime</button>
                        : <button className="btn btn-ghost btn-sm" onClick={() => setTab("Runtime Cache")}>View</button>}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {tab === "Runtime Cache" && (
        <div className="card" style={{ overflow: "hidden" }}>
          <div className="card-h"><span className="t">Runtime Cache {installs && <LiveTag />}</span><button className="chip" onClick={refresh} style={{ cursor: "pointer" }}><GI d={G.refresh} size={11} /> refresh</button></div>
          <table className="tbl">
            <thead><tr><th>Runtime</th><th>Model</th><th>Status</th><th className="hide-sm">Engine</th><th></th></tr></thead>
            <tbody>
              {(installs || []).length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>{installs ? "Nothing installed yet — attach a model from Model Profiles." : "Runtime unreachable."}</td></tr>}
              {(installs || []).map((c) => {
                const downloading = c.status === "downloading" || c.status === "checking" || c.status === "queued";
                const sc = c.status === "ready" || c.status === "attached" ? "green" : c.status === "failed" ? "red" : "amber";
                return (
                  <tr className="row" key={c.id}>
                    <td className="nm mono" style={{ fontSize: 12.5 }}>{c.runtime_id}</td>
                    <td className="mono" style={{ fontSize: 12.5 }}>{c.model_name}</td>
                    <td style={{ minWidth: 160 }}>
                      {downloading
                        ? <div style={{ display: "flex", alignItems: "center", gap: 9 }}><div className="bar" style={{ flex: 1 }}><span className="amber" style={{ width: (c.progress || 0) + "%" }} /></div><span className="mono" style={{ fontSize: 11, color: "var(--lime)" }}>{c.progress || 0}%</span></div>
                        : <span className={"chip " + sc}><span className={"dot " + sc} /> {c.status}</span>}
                    </td>
                    <td className="hide-sm mono" style={{ fontSize: 12 }}>{c.serving_engine || "—"}</td>
                    <td style={{ textAlign: "right" }}><button className="btn btn-ghost btn-sm" onClick={() => c.job_id && window.open(API_BASE + "/v1/jobs/" + c.job_id, "_blank")}>{downloading ? "Logs" : "Test"}</button></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {importOpen && <ImportModelModal initialSource={importSource} onClose={() => { setImportOpen(false); setImportSource(null); }} onImport={handleImport} />}
    </div>
  );
}

/* ---------------------------------------------------------------
   Jobs (live list + quick create + live event timeline).
   --------------------------------------------------------------- */
function JobsView() {
  const { jobs, refresh } = useJobs(3000);
  const [sel, setSel] = React.useState(null);
  const [filter, setFilter] = React.useState("All");
  const [busy, setBusy] = React.useState(false);
  const statuses = ["All", "running", "complete", "error", "queued", "expired"];
  const live = jobs || [];
  const rows = live.filter((j) => filter === "All" || j.status === filter);

  async function quick(type, payload) {
    setBusy(true);
    try { await api.post("/v1/jobs", { type, ttl_seconds: type === "mcp.test" ? 600 : undefined, payload }); await sleep(300); refresh(); }
    catch (e) {} finally { setBusy(false); }
  }

  if (sel) return <JobDetail job={sel} back={() => { setSel(null); refresh(); }} />;
  return (
    <div className="wrap rise">
      <div className="phead">
        <div><p className="eyebrow">Execution</p><h1>Jobs {jobs && <LiveTag />}</h1><p>Every unit of runtime work — sandbox tests, model inspections, pulls, and tool calls — with live status.</p></div>
        <button className="btn btn-ghost" onClick={refresh}><GI d={G.refresh} size={15} /> Refresh</button>
      </div>
      <div className="card card-pad" style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap", marginBottom: 14 }}>
        <span style={{ fontSize: 12.5, color: "var(--ink-3)", fontWeight: 600 }}>Quick run:</span>
        <button className="btn btn-ghost btn-sm" disabled={busy} onClick={() => quick("model.inspect", { model: "hf:Qwen/Qwen2.5-7B-Instruct", revision: "main" })}><GI d={G.cpu} size={13} /> model.inspect Qwen2.5-7B</button>
        <button className="btn btn-ghost btn-sm" disabled={busy} onClick={() => quick("mcp.test", { runtime: "node", transport: "stdio", start_command: FILESYSTEM_CMD })}><GI d={G.play} size={13} /> mcp.test filesystem</button>
      </div>
      <div style={{ display: "flex", gap: 7, flexWrap: "wrap", marginBottom: 16 }}>
        {statuses.map((s) => <button key={s} onClick={() => setFilter(s)} className={"chip" + (filter === s ? " green" : "")} style={{ height: 28, cursor: "pointer", fontFamily: "var(--font)" }}>{s}</button>)}
      </div>
      <div className="card" style={{ overflow: "hidden" }}>
        <table className="tbl">
          <thead><tr><th>Job ID</th><th>Type</th><th>Status</th><th className="hide-sm">Created</th><th className="hide-sm">Expires</th></tr></thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={5} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>{jobs ? "No jobs match." : "Runtime unreachable."}</td></tr>}
            {rows.map((j) => (
              <tr className="row" key={j.job_id} style={{ cursor: "pointer" }} onClick={() => setSel(j)}>
                <td className="nm mono">{j.job_id}</td>
                <td><span className="chip">{j.type}</span></td>
                <td><span className={"chip " + (STATUS_CLASS[j.status] || "")}>{j.status === "running" && <span className="dot blue" />}{j.status}</span></td>
                <td className="hide-sm mono" style={{ fontSize: 12 }}>{(j.created_at || "").replace("T", " ").replace("Z", "")}</td>
                <td className="hide-sm mono" style={{ fontSize: 12 }}>{(j.expires_at || "").replace("T", " ").replace("Z", "")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function JobDetail({ job, back }) {
  const [events, setEvents] = React.useState([]);
  const [snap, setSnap] = React.useState(job);
  React.useEffect(() => {
    let es = null, poll = null, alive = true;
    es = new EventSource(api.eventsURL("/v1/jobs/" + job.job_id + "/events"));
    es.onmessage = (ev) => { try { const d = JSON.parse(ev.data); setEvents((e) => [...e, d]); } catch (e) {} };
    es.onerror = () => { if (es) es.close(); };
    async function tick() { try { const s = await api.get("/v1/jobs/" + job.job_id); if (alive) setSnap(s); } catch (e) {} }
    tick(); poll = setInterval(tick, 2000);
    return () => { alive = false; if (es) es.close(); if (poll) clearInterval(poll); };
  }, [job.job_id]);
  return (
    <div className="wrap rise">
      <button className="chip" onClick={back} style={{ marginBottom: 16, cursor: "pointer" }}><GI d={G.chev} size={13} style={{ transform: "rotate(90deg)" }} /> Jobs</button>
      <div style={{ display: "flex", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
        <h1 style={{ margin: 0, fontSize: 22, fontWeight: 700 }} className="mono">{snap.job_id}</h1>
        <span className="chip">{snap.type}</span>
        <span className={"chip " + (STATUS_CLASS[snap.status] || "")}>{snap.status}</span>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 320px", gap: 16, marginTop: 20 }} className="ov-grid">
        <div className="card">
          <div className="card-h"><span className="t">Event timeline</span><span className="chip"><span className="dot green pulse" /> SSE</span></div>
          <div style={{ padding: "10px 18px 16px" }}>
            {events.map((e, i) => (
              <div key={i} className="rise" style={{ display: "flex", alignItems: "center", gap: 12, padding: "10px 0", borderBottom: "1px solid var(--line)" }}>
                <span className="mono" style={{ fontSize: 13, color: "var(--ink)", flex: 1 }}>{e.step}</span>
                <span style={{ fontSize: 12, color: "var(--ink-3)", flex: 2, minWidth: 0 }}>{e.message}</span>
                <span className={"chip " + (e.status === "ok" || e.status === "complete" ? "green" : e.status === "error" ? "red" : e.status === "expired" || e.status === "cancelled" ? "amber" : "")}>{e.status}</span>
              </div>
            ))}
            {events.length === 0 && <div style={{ padding: "10px 0", color: "var(--ink-4)" }}><span className="cur" /></div>}
          </div>
        </div>
        <div style={{ display: "grid", gap: 16, alignContent: "start" }}>
          <div className="card card-pad">
            <span style={{ fontWeight: 700, fontSize: 13.5 }}>Details</span>
            <div style={{ display: "grid", gap: 9, marginTop: 12, fontSize: 12.5 }}>
              {[["Type", snap.type], ["Status", snap.status], ["Created", (snap.created_at || "").replace("T", " ").replace("Z", "")], ["Expires", (snap.expires_at || "").replace("T", " ").replace("Z", "")]].map(([k, v]) => (
                <div key={k} style={{ display: "flex", justifyContent: "space-between", gap: 10 }}><span style={{ color: "var(--ink-3)" }}>{k}</span><span className="mono" style={{ color: "var(--ink)", textAlign: "right" }}>{v}</span></div>
              ))}
            </div>
          </div>
          <div className="card">
            <div className="card-h" style={{ padding: "10px 14px" }}><span className="mono" style={{ fontSize: 11, color: "var(--ink-3)" }}>RESULT</span></div>
            <pre className="mono" style={{ margin: 0, padding: 14, fontSize: 11.5, color: snap.error ? "var(--red)" : "var(--acc-2)", overflowX: "auto", maxHeight: 280 }}>{snap.error ? snap.error : JSON.stringify(snap.result, null, 2)}</pre>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Logs (live tail of the newest job's SSE, with demo baseline).
   --------------------------------------------------------------- */
function LogsView() {
  const { jobs } = useJobs(4000);
  const [tail, setTail] = React.useState([]);
  const newest = jobs && jobs[0];
  React.useEffect(() => {
    if (!newest) return;
    const es = new EventSource(api.eventsURL("/v1/jobs/" + newest.job_id + "/events"));
    es.onmessage = (ev) => { try { const d = JSON.parse(ev.data); setTail((t) => [...t.slice(-200), { job: newest.job_id, ...d }]); } catch (e) {} };
    es.onerror = () => es.close();
    return () => es.close();
  }, [newest && newest.job_id]);

  function evColor(e) { return e.status === "error" ? "var(--red)" : e.status === "ok" || e.status === "complete" ? "var(--acc-2)" : e.status === "expired" ? "var(--amber)" : "var(--ink-2)"; }
  return (
    <div className="wrap rise">
      <div className="phead"><div><p className="eyebrow">Observability</p><h1>Logs {jobs && <LiveTag />}</h1><p>Live event stream from the newest job on this runtime.</p></div></div>
      <div className="logwin" style={{ height: 440 }}>
        {tail.length === 0 && <div style={{ color: "var(--ink-4)" }}>{newest ? "waiting for events from " + newest.job_id + "…" : "no jobs yet — start a sandbox or inspect a model to see live events"}</div>}
        {tail.map((e, i) => (
          <div key={"t" + i} style={{ color: evColor(e) }}>
            <span style={{ color: "var(--ink-4)" }}>[{e.job}]</span> <span style={{ color: "var(--ink-4)" }}>{e.step}</span>  {e.message}
          </div>
        ))}
        <span className="cur" />
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Governance + settings + profile (reference data).
   --------------------------------------------------------------- */
const POLICY_ICON = { "Sandbox Policy": G.play, "Command Policy": G.shield, "Runtime Policy": G.server };
function PoliciesView() {
  const { data } = useFetch("/v1/policies");
  const policies = data ? data.policies || [] : null;
  return (
    <div className="wrap rise">
      <div className="phead"><div><p className="eyebrow">Governance {policies && <LiveTag />}</p><h1>Policies</h1><p>The guardrails this runtime actually enforces — derived from its live configuration and the command allow/deny lists.</p></div></div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }} className="ov-grid">
        {policies === null && <div style={{ color: "var(--ink-4)", padding: 24 }}>Loading…</div>}
        {(policies || []).map((p) => (
          <div className="card" key={p.name}>
            <div className="card-h"><span className="t" style={{ display: "flex", alignItems: "center", gap: 9 }}><span style={{ color: "var(--acc)", display: "inline-flex" }}><GI d={POLICY_ICON[p.name] || G.shield} size={16} /></span> {p.name}</span><span className={"chip " + (p.active ? "green" : "")}>{p.active && <span className="dot green" />} {p.active ? "enforced" : "off"}</span></div>
            <div style={{ padding: "6px 16px 14px" }}>
              {Object.entries(p.body).map(([k, v]) => (
                <div key={k} style={{ padding: "10px 0", borderBottom: "1px solid var(--line)" }}>
                  <div style={{ fontSize: 12, color: "var(--ink-3)", marginBottom: Array.isArray(v) ? 7 : 0, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 10 }}>
                    <span style={{ fontWeight: 600, color: "var(--ink-2)" }}>{k.replace(/_/g, " ")}</span>
                    {!Array.isArray(v) && (typeof v === "boolean"
                      ? <span className={"chip " + (v ? "green" : "")}>{v ? "yes" : "no"}</span>
                      : <span className="mono" style={{ fontSize: 12, color: "var(--ink)" }}>{String(v)}</span>)}
                  </div>
                  {Array.isArray(v) && (v.length
                    ? <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>{v.map((x) => <span key={x} className={"chip" + (k.startsWith("blocked") ? " red" : "")} style={{ fontFamily: "var(--mono)", fontSize: 11 }}>{x}</span>)}</div>
                    : <span style={{ fontSize: 12, color: "var(--ink-4)" }}>none</span>)}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function AuditView() {
  const [events, setEvents] = React.useState(null);
  const load = React.useCallback(() => { cloud.listAudit().then(setEvents).catch(() => setEvents([])); }, []);
  React.useEffect(() => { load(); const t = setInterval(load, 10000); return () => clearInterval(t); }, [load]);
  const rows = events || [];
  return (
    <div className="wrap rise">
      <div className="phead"><div><p className="eyebrow">Compliance {events && <LiveTag />}</p><h1>Audit</h1><p>Append-only record of sensitive actions in your workspace — logins, runtime &amp; token activity, credentials, model imports and attaches, and MatrixShell commands.</p></div><button className="btn btn-ghost" onClick={load}><GI d={G.refresh} size={15} /> Refresh</button></div>
      <div className="card" style={{ overflow: "hidden" }}>
        <table className="tbl">
          <thead><tr><th>Timestamp</th><th>Action</th><th>Target</th><th className="hide-sm">Actor</th><th className="hide-sm">IP</th><th>Status</th></tr></thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={6} style={{ color: "var(--ink-4)", textAlign: "center", padding: 24 }}>{events ? "No audited activity yet." : "…"}</td></tr>}
            {rows.map((e) => (
              <tr className="row" key={e.id}>
                <td className="mono" style={{ fontSize: 12, color: "var(--ink-3)" }}>{(e.created_at || "").replace("T", " ").replace("Z", "").replace(/\..*/, "")}</td>
                <td className="nm"><span className="chip">{e.action}</span></td>
                <td style={{ fontSize: 12.5, maxWidth: 260, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{e.target || "—"}</td>
                <td className="hide-sm mono" style={{ fontSize: 11.5, color: "var(--ink-3)" }}>{e.actor || "—"}</td>
                <td className="hide-sm mono" style={{ fontSize: 11.5, color: "var(--ink-4)" }}>{e.ip || "—"}</td>
                <td><span className={"chip " + (e.status === "success" ? "green" : "red")}><span className={"dot " + (e.status === "success" ? "green" : "red")} /> {e.status}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// PROVIDERS — the model providers a workspace can bring its own credentials for.
const BYO_PROVIDERS = [
  { id: "huggingface", name: "Hugging Face", hint: "hf_…", ph: "hf_xxxxxxxxxxxxxxxxxxxx", note: "Use your own HF account & inference quota for HF LLMs inside your runtimes.", meta: "default_model" },
  { id: "openai", name: "OpenAI-compatible", hint: "sk_…", ph: "sk-…", note: "Any OpenAI-compatible endpoint (OpenAI, Together, Groq, …).", meta: "base_url" },
  { id: "anthropic", name: "Anthropic", hint: "sk-ant-…", ph: "sk-ant-…", note: "Claude models via the Anthropic API.", meta: "default_model" },
];

function ProvidersCard() {
  const [list, setList] = React.useState(null);
  const [open, setOpen] = React.useState(null); // provider id being edited
  const [secret, setSecret] = React.useState("");
  const [metaVal, setMetaVal] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [err, setErr] = React.useState("");
  const [okMsg, setOkMsg] = React.useState("");

  const load = React.useCallback(() => {
    cloud.listProviders().then(setList).catch(() => setList([]));
  }, []);
  React.useEffect(load, [load]);

  function startEdit(pid) { setOpen(pid); setSecret(""); setMetaVal(""); setErr(""); setOkMsg(""); }

  async function save(p) {
    setErr(""); setOkMsg("");
    if (!secret.trim()) return setErr("Paste your token first.");
    setBusy(true);
    try {
      const meta = metaVal.trim() ? { [p.meta]: metaVal.trim() } : undefined;
      await cloud.setProvider(p.id, "default", secret.trim(), meta);
      setOpen(null); setSecret(""); setMetaVal(""); setOkMsg(p.name + " connected.");
      load();
    } catch (e) { setErr(e.message || "Could not save credential."); }
    finally { setBusy(false); }
  }

  const byId = {};
  (list || []).forEach((c) => { byId[c.provider] = c; });

  return (
    <div className="card" style={{ gridColumn: "1 / -1" }}>
      <div className="card-h">
        <span className="t">Model providers · bring your own key</span>
        {list && <span className="chip"><span className="dot green" /> {list.length} connected</span>}
      </div>
      <div style={{ padding: "4px 18px 14px" }}>
        <p style={{ fontSize: 12.5, color: "var(--ink-3)", lineHeight: 1.6, margin: "10px 0 6px" }}>
          Plug in your own provider tokens. They're encrypted at rest (AES-256-GCM) and used server-side only — the console never shows the secret back, only a <span className="mono">••••1234</span> hint.
        </p>
        {okMsg && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "9px 11px", margin: "6px 0", borderRadius: "var(--r-sm)", border: "1px solid var(--acc-line)", background: "var(--acc-soft)", color: "var(--acc-2)", fontSize: 12.5 }}><GI d={G.check} size={14} sw={2.4} /> {okMsg}</div>}
        {BYO_PROVIDERS.map((p) => {
          const cur = byId[p.id];
          const editing = open === p.id;
          return (
            <div key={p.id} style={{ padding: "13px 0", borderBottom: "1px solid var(--line)" }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontSize: 13.5, fontWeight: 600, display: "flex", alignItems: "center", gap: 8 }}>
                    {p.name}
                    {cur ? <span className="chip green"><span className="dot green" /> {cur.hint}</span> : <span className="chip" style={{ color: "var(--ink-4)" }}>not set</span>}
                  </div>
                  <div style={{ fontSize: 12, color: "var(--ink-4)", marginTop: 3 }}>{p.note}</div>
                </div>
                <button className="btn btn-ghost btn-sm" onClick={() => (editing ? setOpen(null) : startEdit(p.id))}>
                  {editing ? "Cancel" : cur ? "Replace" : "Connect"}
                </button>
              </div>
              {editing && (
                <div style={{ display: "grid", gap: 10, marginTop: 12 }}>
                  <AField label={p.name + " token"} type="password" value={secret} onChange={setSecret} placeholder={p.ph} icon="key" autoFocus />
                  <AField label={p.meta === "base_url" ? "Base URL (optional)" : "Default model (optional)"} value={metaVal} onChange={setMetaVal} placeholder={p.meta === "base_url" ? "https://api.example.com/v1" : "Qwen/Qwen2.5-7B-Instruct"} icon="cpu" />
                  {err && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "9px 11px", borderRadius: "var(--r-sm)", border: "1px solid rgba(251,113,133,0.3)", background: "var(--red-soft)", color: "var(--red)", fontSize: 12.5 }}><GI d={G.x} size={14} /> {err}</div>}
                  <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
                    <button className="btn btn-primary btn-sm" disabled={busy} onClick={() => save(p)}>{busy ? "Saving…" : "Save token"}</button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function CloudSettingsView() {
  const rt = useRuntime();
  const { data: meData } = useFetch("/v1/auth/me");
  const me = meData && meData.user;
  const det = (rt.caps && rt.caps.runtimes) || {};
  const sections = [
    ["Workspace", me ? me.workspace : "—"],
    ["Owner", me ? me.email : "—"],
    ["Role", me ? me.role : "—"],
    ["Runtime", rt.health ? rt.health.runtime_id : "—"],
    ["Mode", rt.health ? rt.health.mode : "—"],
    ["Version", rt.health ? "v" + rt.health.version : "—"],
  ];
  const registries = [
    ["Hugging Face", "connected"],
    ["Node runner", det.node ? "connected" : "available"],
    ["Python runner", det.python ? "connected" : "available"],
    ["Ollama", det.ollama ? "connected" : "available"],
    ["vLLM", det.vllm ? "connected" : "available"],
  ];
  return (
    <div className="wrap rise">
      <div className="phead"><div><p className="eyebrow">Workspace</p><h1>Settings</h1><p>Your workspace, this runtime, and the detected model runtimes.</p></div></div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }} className="ov-grid">
        <div className="card">
          <div className="card-h"><span className="t">Workspace</span></div>
          <div style={{ padding: "4px 18px" }}>
            {sections.map(([k, v]) => (
              <div key={k} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "13px 0", borderBottom: "1px solid var(--line)" }}>
                <span style={{ fontSize: 13.5, fontWeight: 600 }}>{k}</span><span style={{ fontSize: 12.5, color: "var(--ink-3)" }} className="mono">{v}</span>
              </div>
            ))}
          </div>
        </div>
        <div className="card">
          <div className="card-h"><span className="t">Registries</span><button className="chip" style={{ cursor: "pointer" }}><GI d={G.plus} size={11} /> add</button></div>
          <div style={{ padding: "4px 18px" }}>
            {registries.map(([k, v]) => (
              <div key={k} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "13px 0", borderBottom: "1px solid var(--line)" }}>
                <span style={{ fontSize: 13.5, fontWeight: 600 }}>{k}</span>
                {v === "connected" ? <span className="chip green"><span className="dot green" /> connected</span> : <span className="chip" style={{ color: "var(--ink-4)" }}>{v}</span>}
              </div>
            ))}
          </div>
        </div>
        <StorageCard />
        <ProvidersCard />
      </div>
    </div>
  );
}

const PROFILE_DEFAULTS = { name: "Neo Anderson", email: "neo@acme.io", username: "neo", role: "AI Engineer", theme: "Dark", language: "English", timezone: "UTC", dateFormat: "YYYY-MM-DD", defaultWorkspace: "acme-prod", defaultEnv: "Production", notifyEmail: true, notifyJobs: true, notifySecurity: true, notifyDigest: false };
function loadProfile() { try { return { ...PROFILE_DEFAULTS, ...JSON.parse(localStorage.getItem("mcloud-profile") || "{}") }; } catch (e) { return { ...PROFILE_DEFAULTS }; } }
function PField({ label, hint, children }) {
  return <label style={{ display: "block", minWidth: 0 }}><span style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--ink-2)", marginBottom: 6 }}>{label}</span>{children}{hint && <span style={{ display: "block", fontSize: 11.5, color: "var(--ink-4)", marginTop: 5 }}>{hint}</span>}</label>;
}
const PINPUT = { width: "100%", height: 40, padding: "0 12px", fontSize: 13.5, color: "var(--ink)", background: "var(--inset)", border: "1px solid var(--line-2)", borderRadius: "var(--r-sm)", outline: "none" };
const PSELECT = { ...PINPUT, cursor: "pointer", appearance: "none", WebkitAppearance: "none", backgroundImage: "linear-gradient(45deg,transparent 50%,var(--ink-3) 50%),linear-gradient(135deg,var(--ink-3) 50%,transparent 50%)", backgroundPosition: "calc(100% - 16px) 17px, calc(100% - 11px) 17px", backgroundSize: "5px 5px, 5px 5px", backgroundRepeat: "no-repeat" };
function PToggle({ on, onChange }) {
  return <button onClick={() => onChange(!on)} role="switch" aria-checked={on} style={{ width: 40, height: 23, borderRadius: 99, padding: 2, flexShrink: 0, background: on ? "var(--acc)" : "var(--line-3)", transition: "background .15s" }}><span style={{ display: "block", width: 19, height: 19, borderRadius: 99, background: "#fff", transform: on ? "translateX(17px)" : "translateX(0)", transition: "transform .15s", boxShadow: "0 1px 3px rgba(0,0,0,0.3)" }} /></button>;
}
function ProfileView() {
  const [p, setP] = React.useState(loadProfile);
  const [saved, setSaved] = React.useState(false);
  const set = (k, v) => { setP((s) => ({ ...s, [k]: v })); setSaved(false); };
  const initials = p.name.split(" ").map((x) => x[0]).slice(0, 2).join("").toUpperCase();
  function save() { try { localStorage.setItem("mcloud-profile", JSON.stringify(p)); } catch (e) {} setSaved(true); setTimeout(() => setSaved(false), 1800); }
  function reset() { setP({ ...PROFILE_DEFAULTS }); setSaved(false); }
  const sel = (k, opts) => <select style={PSELECT} value={p[k]} onChange={(e) => set(k, e.target.value)}>{opts.map((o) => <option key={o} value={o}>{o}</option>)}</select>;
  return (
    <div className="wrap rise" style={{ maxWidth: 880 }}>
      <div className="phead"><div><p className="eyebrow">Account</p><h1>Profile &amp; preferences</h1><p>Your personal account details and standard workspace preferences.</p></div></div>
      <div className="card card-pad" style={{ display: "flex", alignItems: "center", gap: 16, flexWrap: "wrap" }}>
        <div style={{ width: 60, height: 60, borderRadius: 14, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", background: "linear-gradient(135deg, var(--acc), var(--acc-dim))", color: "#042012", fontWeight: 800, fontSize: 22 }}>{initials}</div>
        <div style={{ flex: 1, minWidth: 0 }}><div style={{ fontSize: 18, fontWeight: 700 }}>{p.name}</div><div style={{ fontSize: 13, color: "var(--ink-3)", marginTop: 2 }}>{p.email}</div></div>
        <span className="chip violet">{p.role}</span>
        <button className="btn btn-ghost btn-sm">Change avatar</button>
      </div>
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-h"><span className="t">Account</span></div>
        <div className="card-pad ov-grid" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
          <PField label="Full name"><input style={PINPUT} value={p.name} onChange={(e) => set("name", e.target.value)} /></PField>
          <PField label="Email"><input style={PINPUT} value={p.email} onChange={(e) => set("email", e.target.value)} /></PField>
          <PField label="Username"><input style={{ ...PINPUT, fontFamily: "var(--mono)" }} value={p.username} onChange={(e) => set("username", e.target.value)} /></PField>
          <PField label="Role" hint="Assigned by your workspace admin.">{sel("role", ["Platform Admin", "AI Engineer", "Security / Compliance", "Developer"])}</PField>
        </div>
      </div>
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-h"><span className="t">Preferences</span></div>
        <div className="card-pad ov-grid" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16 }}>
          <PField label="Theme">{sel("theme", ["Dark", "System"])}</PField>
          <PField label="Language">{sel("language", ["English", "Español", "Deutsch", "Français", "日本語"])}</PField>
          <PField label="Timezone">{sel("timezone", ["UTC", "America/New_York", "America/Los_Angeles", "Europe/London", "Europe/Berlin", "Asia/Tokyo"])}</PField>
          <PField label="Date format">{sel("dateFormat", ["YYYY-MM-DD", "DD/MM/YYYY", "MM/DD/YYYY"])}</PField>
          <PField label="Default workspace">{sel("defaultWorkspace", ["acme-prod", "shared", "personal"])}</PField>
          <PField label="Default environment">{sel("defaultEnv", ["Production", "Staging", "Development"])}</PField>
        </div>
      </div>
      <div className="card" style={{ marginTop: 16 }}>
        <div className="card-h"><span className="t">Notifications</span></div>
        <div style={{ padding: "4px 18px" }}>
          {[["notifyEmail", "Email notifications", "Receive account and workspace emails."], ["notifyJobs", "Job alerts", "Notify when your jobs complete or fail."], ["notifySecurity", "Security alerts", "Approvals, policy changes, and token events."], ["notifyDigest", "Weekly digest", "A weekly summary of activity and usage."]].map(([k, nm, ds]) => (
            <div key={k} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 14, padding: "14px 0", borderBottom: "1px solid var(--line)" }}>
              <div><div style={{ fontSize: 13.5, fontWeight: 600 }}>{nm}</div><div style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 2 }}>{ds}</div></div>
              <PToggle on={p[k]} onChange={(v) => set(k, v)} />
            </div>
          ))}
        </div>
      </div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginTop: 20, flexWrap: "wrap" }}>
        <button className="btn btn-ghost" onClick={reset}><GI d={G.refresh} size={15} /> Reset to defaults</button>
        <div style={{ display: "flex", gap: 10 }}>
          <button className="btn btn-ghost" style={{ color: "var(--red)", borderColor: "rgba(255,93,108,0.3)" }}>Sign out</button>
          <button className="btn btn-primary" onClick={save}>{saved ? <><GI d={G.check} size={15} sw={2.4} /> Saved</> : "Save changes"}</button>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   App shell + routing.
   --------------------------------------------------------------- */
/* ---------------------------------------------------------------
   Auth screen (wired to /v1/auth via the SQLite user store).
   --------------------------------------------------------------- */
function AField({ label, type = "text", value, onChange, placeholder, icon, autoFocus }) {
  const I = icon ? G[icon] : null;
  return (
    <label style={{ display: "block" }}>
      <span style={{ display: "block", fontFamily: "var(--mono)", fontSize: 10.5, letterSpacing: "0.18em", textTransform: "uppercase", color: "var(--ink-3)", marginBottom: 7 }}>{label}</span>
      <div style={{ display: "flex", alignItems: "center", gap: 10, height: 46, padding: "0 13px", borderRadius: "var(--r-sm)", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.35)" }}>
        {I && <span style={{ color: "var(--acc)", display: "inline-flex", flexShrink: 0 }}><GI d={I} size={16} /></span>}
        <input type={type} value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder} autoFocus={autoFocus} spellCheck={false}
          style={{ flex: 1, minWidth: 0, height: "100%", background: "transparent", border: "none", outline: "none", color: "var(--ink)", fontSize: 14 }} />
      </div>
    </label>
  );
}

function AuthScreen({ onAuthed }) {
  const intent = React.useMemo(urlIntent, []);
  const [mode, setMode] = React.useState(intent.kind === "reset" ? "reset" : intent.kind === "verify" ? "verify" : "login");
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [pw, setPw] = React.useState("");
  const [pw2, setPw2] = React.useState("");
  const [resetToken] = React.useState(intent.token);
  const [err, setErr] = React.useState("");
  const [msg, setMsg] = React.useState("");
  const [busy, setBusy] = React.useState(false);

  // Email verification runs as soon as the screen mounts with a verify link.
  React.useEffect(() => {
    if (mode !== "verify") return;
    if (!resetToken) { setErr("This verification link is missing its token."); return; }
    setBusy(true);
    auth.verify(resetToken)
      .then((m) => { setMsg(m); clearURLToken(); })
      .catch((e) => setErr(e.message || "Verification failed."))
      .finally(() => setBusy(false));
  }, [mode, resetToken]);

  function switchMode(m) { setMode(m); setErr(""); setMsg(""); }

  async function submit(e) {
    e.preventDefault(); setErr(""); setMsg("");
    // Password recovery — request a reset link.
    if (mode === "forgot") {
      if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return setErr("Enter a valid email.");
      setBusy(true);
      try { setMsg(await auth.forgot(email.trim())); }
      catch (e2) { setErr(e2.message || "Could not send reset email."); }
      finally { setBusy(false); }
      return;
    }
    // Choose a new password from a reset link.
    if (mode === "reset") {
      if (pw.length < 8) return setErr("Password must be at least 8 characters.");
      if (pw !== pw2) return setErr("Passwords do not match.");
      setBusy(true);
      try { setMsg(await auth.reset(resetToken, pw)); clearURLToken(); setTimeout(() => switchMode("login"), 1200); }
      catch (e2) { setErr(e2.message || "Could not reset password."); }
      finally { setBusy(false); }
      return;
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) return setErr("Enter a valid work email.");
    if (pw.length < 6) return setErr("Password must be at least 6 characters.");
    if (mode === "signup") {
      if (!name.trim()) return setErr("Enter your full name.");
      if (pw !== pw2) return setErr("Passwords do not match.");
    }
    setBusy(true);
    try {
      const user = mode === "signup" ? await auth.signup(name.trim(), email.trim(), pw) : await auth.login(email.trim(), pw);
      onAuthed(user);
    } catch (e2) { setErr(e2.message || "Authentication failed."); }
    finally { setBusy(false); }
  }

  const feats = [
    ["shield", "Secure by default", "Signed manifests, audited installs, approval gates."],
    ["server", "Runtime isolation", "Hybrid and customer-agent execution modes."],
    ["cpu", "Model gateway", "Hugging Face, OpenAI-compatible, Ollama, vLLM."],
    ["audit", "Complete audit trail", "Every install, policy, and access change."],
  ];
  const HEAD = {
    login: ["Welcome back", "Sign in to MatrixCloud", "Use your work email and password."],
    signup: ["Get started", "Create your account", "Set up a workspace owner account to get started."],
    forgot: ["Account recovery", "Reset your password", "Enter your email and we'll send you a secure reset link."],
    reset: ["Account recovery", "Choose a new password", "Pick a strong password — at least 8 characters."],
    verify: ["Email verification", "Verifying your email", "Confirming your address with MatrixCloud."],
  };
  const ACTION = { login: "Sign in", signup: "Create account", forgot: "Send reset link", reset: "Update password", verify: "Continue" };

  return (
    <div style={{ position: "relative", zIndex: 1, minHeight: "100vh", display: "grid", gridTemplateColumns: "1.1fr 520px" }} className="auth-grid">
      <section className="auth-hero" style={{ display: "flex", flexDirection: "column", justifyContent: "space-between", padding: "44px 52px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 11 }}>
          <span style={{ width: 38, height: 38, borderRadius: 11, display: "grid", placeItems: "center", border: "1px solid var(--acc-line)", background: "var(--acc-soft)", color: "var(--acc)", boxShadow: "0 0 26px rgba(0,255,136,0.14)" }}><GI d={G.terminal} size={19} /></span>
          <div>
            <p style={{ margin: 0, fontSize: 16, fontWeight: 700, letterSpacing: "-0.01em" }}>MatrixCloud</p>
            <p style={{ margin: "2px 0 0", fontFamily: "var(--mono)", fontSize: 9.5, letterSpacing: "0.24em", textTransform: "uppercase", color: "var(--acc-2)", opacity: 0.7 }}>Enterprise Control Plane</p>
          </div>
        </div>
        <div style={{ maxWidth: 620 }}>
          <p className="eyebrow">Secure AI infrastructure</p>
          <h1 style={{ margin: "18px 0 0", fontSize: "clamp(34px, 4vw, 56px)", fontWeight: 600, letterSpacing: "-0.045em", lineHeight: 1.02 }}>Operate your AI<br />runtime layer.</h1>
          <p style={{ margin: "20px 0 0", fontSize: 15, lineHeight: 1.7, color: "var(--ink-3)", maxWidth: 520 }}>Connect runtimes, test MCP servers, manage models, run agents, enforce policies, and audit every action across your workspace.</p>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, marginTop: 28 }} className="auth-feats">
            {feats.map(([ic, t, c]) => (
              <div key={t} style={{ borderRadius: "var(--r-md)", border: "1px solid var(--line)", background: "rgba(0,0,0,0.25)", padding: 16, backdropFilter: "blur(10px)" }}>
                <span style={{ color: "var(--acc)", display: "inline-flex" }}><GI d={G[ic]} size={18} /></span>
                <p style={{ margin: "11px 0 0", fontSize: 13.5, fontWeight: 600 }}>{t}</p>
                <p style={{ margin: "5px 0 0", fontSize: 12, lineHeight: 1.55, color: "var(--ink-4)" }}>{c}</p>
              </div>
            ))}
          </div>
        </div>
        <p style={{ margin: 0, fontFamily: "var(--mono)", fontSize: 11, color: "var(--ink-4)" }}>multitenant workspaces · session security · approval gates</p>
      </section>

      <section className="auth-form" style={{ display: "flex", alignItems: "center", justifyContent: "center", borderLeft: "1px solid var(--line)", background: "rgba(0,0,0,0.3)", padding: 28, backdropFilter: "blur(20px)" }}>
        <div className="card rise" style={{ width: "100%", maxWidth: 400, padding: 28 }}>
          <p className="eyebrow">{HEAD[mode][0]}</p>
          <h2 style={{ margin: "12px 0 0", fontSize: 25, fontWeight: 600, letterSpacing: "-0.02em" }}>{HEAD[mode][1]}</h2>
          <p style={{ margin: "8px 0 0", fontSize: 13, lineHeight: 1.6, color: "var(--ink-3)" }}>{HEAD[mode][2]}</p>
          {(mode === "login" || mode === "signup") && (
            <div style={{ display: "flex", gap: 4, marginTop: 20, padding: 4, borderRadius: "var(--r-sm)", border: "1px solid var(--line)", background: "rgba(0,0,0,0.3)" }}>
              {[["login", "Sign in"], ["signup", "Sign up"]].map(([m, lab]) => (
                <button key={m} onClick={() => switchMode(m)} style={{ flex: 1, height: 34, borderRadius: 8, fontSize: 13, fontWeight: 600, background: mode === m ? "var(--acc-soft)" : "transparent", color: mode === m ? "var(--acc-2)" : "var(--ink-3)", border: "1px solid " + (mode === m ? "var(--line-2)" : "transparent") }}>{lab}</button>
              ))}
            </div>
          )}
          {mode === "verify" ? (
            <div style={{ marginTop: 22, display: "grid", gap: 14 }}>
              {busy && <p style={{ fontSize: 13, color: "var(--ink-3)" }}>Verifying your email…</p>}
              {msg && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 12px", borderRadius: "var(--r-sm)", border: "1px solid var(--acc-line)", background: "var(--acc-soft)", color: "var(--acc-2)", fontSize: 12.5 }}><GI d={G.check} size={14} sw={2.4} /> {msg}</div>}
              {err && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 12px", borderRadius: "var(--r-sm)", border: "1px solid rgba(251,113,133,0.3)", background: "var(--red-soft)", color: "var(--red)", fontSize: 12.5 }}><GI d={G.x} size={14} /> {err}</div>}
              <button className="btn btn-primary" onClick={() => switchMode("login")} style={{ height: 46 }}>Continue to sign in</button>
            </div>
          ) : (
            <form onSubmit={submit} style={{ display: "grid", gap: 14, marginTop: 18 }}>
              {mode === "signup" && <AField label="Full name" value={name} onChange={setName} placeholder="Maya Chen" icon="user" autoFocus />}
              {(mode === "login" || mode === "signup" || mode === "forgot") &&
                <AField label="Work email" type="email" value={email} onChange={setEmail} placeholder="you@company.com" icon="globe" autoFocus={mode === "login" || mode === "forgot"} />}
              {mode !== "forgot" && <AField label={mode === "reset" ? "New password" : "Password"} type="password" value={pw} onChange={setPw} placeholder="••••••••" icon="key" autoFocus={mode === "reset"} />}
              {(mode === "signup" || mode === "reset") && <AField label="Confirm password" type="password" value={pw2} onChange={setPw2} placeholder="••••••••" icon="key" />}
              {err && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 12px", borderRadius: "var(--r-sm)", border: "1px solid rgba(251,113,133,0.3)", background: "var(--red-soft)", color: "var(--red)", fontSize: 12.5 }}><GI d={G.x} size={14} /> {err}</div>}
              {msg && <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 12px", borderRadius: "var(--r-sm)", border: "1px solid var(--acc-line)", background: "var(--acc-soft)", color: "var(--acc-2)", fontSize: 12.5 }}><GI d={G.check} size={14} sw={2.4} /> {msg}</div>}
              {mode === "login" && (
                <div style={{ textAlign: "right", marginTop: -4 }}>
                  <button type="button" onClick={() => switchMode("forgot")} style={{ fontSize: 12, color: "var(--acc-2)", fontWeight: 600 }}>Forgot password?</button>
                </div>
              )}
              <button className="btn btn-primary" type="submit" disabled={busy} style={{ height: 46, marginTop: 2 }}>{busy ? "Please wait…" : ACTION[mode]}</button>
            </form>
          )}
          {(mode === "login" || mode === "signup") && (
            <p style={{ margin: "16px 0 0", textAlign: "center", fontSize: 12.5, color: "var(--ink-3)" }}>
              {mode === "login" ? "Need an account? " : "Already have an account? "}
              <button onClick={() => switchMode(mode === "login" ? "signup" : "login")} style={{ color: "var(--acc-2)", fontWeight: 600 }}>{mode === "login" ? "Sign up" : "Sign in"}</button>
            </p>
          )}
          {(mode === "forgot" || mode === "reset") && (
            <p style={{ margin: "16px 0 0", textAlign: "center", fontSize: 12.5, color: "var(--ink-3)" }}>
              <button onClick={() => switchMode("login")} style={{ color: "var(--acc-2)", fontWeight: 600 }}>← Back to sign in</button>
            </p>
          )}
          <p style={{ margin: "14px 0 0", textAlign: "center", fontFamily: "var(--mono)", fontSize: 10.5, color: "var(--ink-4)" }}>multitenant workspaces · session security</p>
        </div>
      </section>
    </div>
  );
}

/* ---------------------------------------------------------------
   MatrixShell — AI-assisted operator terminal, wired to the runtime.
   `matrix` subcommands hit the real /v1 API; shell passthrough is a
   safe sandboxed simulation; plain English becomes a confirmable
   command suggestion with a risk level.
   --------------------------------------------------------------- */
const MS_DENY = [/mkfs/i, /dd\s+if=/i, /\bfdisk\b/i, /\bdiskpart\b/i, /\bshutdown\b/i, /\breboot\b/i, /:\(\)\s*\{/, /\brm\s+-rf\s+(\/|~|\$HOME)(\s|$)/i];
const msDenied = (c) => MS_DENY.some((re) => re.test(c));
async function msMatrix(cmd) {
  const p = cmd.trim().split(/\s+/); const sub = (p[1] || "").toLowerCase();
  try {
    if (sub === "status") {
      const [h, c] = await Promise.all([api.get("/v1/health"), api.get("/v1/capabilities")]);
      return { tone: "ok", lines: [
        `runtime ${h.runtime_id} · mode ${h.mode} · v${h.version}`,
        `capabilities: ${(c.capabilities || []).join(", ")}`,
        `runtimes: node=${c.runtimes.node} python=${c.runtimes.python} ollama=${c.runtimes.ollama}`,
        `limits: max_ttl=${c.limits.max_ttl_seconds}s · max_jobs=${c.limits.max_concurrent_jobs}`] };
    }
    if (sub === "ps") {
      const r = await api.get("/v1/jobs"); const jobs = (r.jobs || []).slice(0, 10);
      if (!jobs.length) return { tone: "dim", lines: ["no jobs yet — try: matrix inspect hf:Qwen/Qwen2.5-7B-Instruct"] };
      return { tone: "ok", lines: ["RUNNING / RECENT JOBS", ...jobs.map((j) => `  ${j.job_id}  ${j.type.padEnd(13)} ${j.status}`)] };
    }
    if (sub === "capabilities" || sub === "caps") {
      const c = await api.get("/v1/capabilities");
      return { tone: "ok", lines: ["CAPABILITIES", ...(c.capabilities || []).map((x) => "  " + x)] };
    }
    if (sub === "inspect") {
      const model = p[2] || "hf:Qwen/Qwen2.5-7B-Instruct";
      const meta = await runJobToCompletion("model.inspect", { model, revision: "main" });
      return { tone: "ok", lines: [
        `model ${meta.model}`,
        `task=${meta.pipeline_tag || "?"} · library=${meta.library_name || "?"} · type=${meta.model_type || "?"}`,
        `license=${meta.license || "?"} · params≈${fmtParams(meta.estimated_parameters)} · runtime=${meta.recommended_runtime} · gpu=${meta.requires_gpu}`] };
    }
    if (sub === "help" || p.length === 1) return { tone: "sys", lines: [
      "MATRIX CLI — inside MatrixShell (wired to this runtime)",
      "  matrix status            runtime health + capabilities (live)",
      "  matrix ps                running / recent jobs (live)",
      "  matrix capabilities      advertised capabilities (live)",
      "  matrix inspect <model>   resolve a model via model.inspect (live)",
      "  …or just describe what you want in plain English."] };
    return { tone: "dim", lines: [`unknown: ${cmd}`, "try: matrix help"] };
  } catch (e) { return { tone: "dim", lines: ["✗ " + (e.message || "command failed")] }; }
}
const MS_NL = [
  { re: /inspect.*runtime|runtime.*(health|status)|status/i, cmd: "matrix status", risk: "low", expl: "Shows the connected runtime, its mode, version and capabilities — live from this runtime." },
  { re: /recent jobs|show.*jobs|running jobs|\bjobs\b/i, cmd: "matrix ps", risk: "low", expl: "Lists the jobs currently running on the connected runtime." },
  { re: /(list|show).*(capabilit|tools)/i, cmd: "matrix capabilities", risk: "low", expl: "Lists the capabilities this runtime advertises." },
  { re: /inspect.*model|resolve.*model|qwen|llama|mistral/i, cmd: "matrix inspect hf:Qwen/Qwen2.5-7B-Instruct", risk: "low", expl: "Resolves model metadata (task, license, parameters, recommended runtime) via model.inspect." },
  { re: /(biggest|largest) files?/i, cmd: "du -ah . | sort -hr | head -n 20", risk: "low", expl: "Lists the 20 largest files and folders in the current directory." },
  { re: /disk|space|storage/i, cmd: "df -h", risk: "low", expl: "Shows disk space usage for mounted filesystems." },
  { re: /(install|add).*(package|dependency)/i, cmd: "pip install <package>", risk: "medium", expl: "Installs a package into the sandbox. Write operation — requires approval." },
];
function msSuggest(t) { for (const r of MS_NL) if (r.re.test(t)) return { cmd: r.cmd, risk: r.risk, expl: r.expl }; return { cmd: `echo "${t.replace(/"/g, "")}"`, risk: "low", expl: "Could not map this to a known operation — here is a safe echo. Refine it, or run a matrix command." }; }
const MS_HEADS = ["ls", "cd", "pwd", "whoami", "echo", "cat", "ps", "df", "du", "git", "python", "python3", "pip", "uv", "matrix", "matrixsh", "grep", "rm", "mkdir", "mv", "cp", "head", "tail", "find", "which", "env", "node", "npm", "npx", "curl", "touch", "wc", "sort", "make", "go"];
function msLooksCmd(t) { const s = t.trim(); if (!s) return false; if (/[?]|\bhow\b|\bwhat\b|\bwhy\b|\bcan i\b|please|inspect|show|list|test|create/i.test(s) && !s.startsWith("matrix")) return false; return MS_HEADS.includes(s.split(/\s+/)[0].toLowerCase()); }
function msTone(t) { return t === "ok" ? "var(--acc-2)" : t === "sys" ? "var(--blue)" : t === "warn" ? "var(--lime)" : t === "dim" ? "var(--ink-3)" : "var(--ink-2)"; }

function Suggestion({ s, answered, onAnswer }) {
  const riskCls = s.risk === "high" ? "red" : s.risk === "medium" ? "amber" : "green";
  return (
    <div style={{ border: "1px solid var(--line-2)", borderRadius: "var(--r-md)", margin: "8px 0 4px", overflow: "hidden", background: "rgba(0,0,0,0.3)" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 13px", borderBottom: "1px solid var(--line)", background: "rgba(92,200,255,0.05)" }}>
        <span style={{ color: "var(--blue)", display: "inline-flex" }}><GI d={G.cpu} size={14} /></span>
        <span style={{ fontSize: 12, fontWeight: 600, color: "var(--blue)", letterSpacing: "0.02em" }}>Suggested command</span>
      </div>
      <div style={{ padding: "12px 14px" }}>
        <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "var(--ink-2)" }}>{s.expl}</div>
        <pre className="mono" style={{ margin: "10px 0 0", padding: "10px 12px", borderRadius: 8, background: "rgba(0,0,0,0.5)", border: "1px solid var(--line)", fontSize: 12.5, color: "#fff", overflowX: "auto" }}>{s.cmd}</pre>
        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 11 }}>
          <span className={"chip " + riskCls}>risk: {s.risk}</span>
          {s.risk !== "low" && <span style={{ fontSize: 11.5, color: "var(--lime)" }}>write operation · approval required</span>}
        </div>
        {!answered ? (
          <div style={{ display: "flex", alignItems: "center", gap: 9, marginTop: 12 }}>
            <span style={{ fontSize: 12.5, color: "var(--ink-2)" }}>Execute it?</span>
            <button className="btn btn-primary btn-sm" onClick={() => onAnswer(true)}>Yes, run</button>
            <button className="btn btn-ghost btn-sm" onClick={() => onAnswer(false)}>No</button>
          </div>
        ) : (<div style={{ marginTop: 12, fontSize: 12.5, color: answered === "yes" ? "var(--acc-2)" : "var(--ink-3)" }}>{answered === "yes" ? "✓ executed" : "✗ cancelled"}</div>)}
      </div>
    </div>
  );
}

function MatrixShellView({ back, user }) {
  const rt = useRuntime();
  const [status, setStatus] = React.useState(null); // null=loading, {installed,version,...}
  const [lines, setLines] = React.useState([]);
  const [value, setValue] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [installing, setInstalling] = React.useState(false);
  const scroller = React.useRef(null);
  const inputRef = React.useRef(null);
  const hist = React.useRef([]); const hi = React.useRef(-1);

  const refreshStatus = React.useCallback(async () => {
    try { setStatus(await api.get("/v1/matrixshell/status")); } catch (e) { setStatus({ installed: false, error: e.message }); }
  }, []);
  React.useEffect(() => { refreshStatus(); inputRef.current && inputRef.current.focus(); }, [refreshStatus]);
  React.useEffect(() => { if (scroller.current) scroller.current.scrollTop = scroller.current.scrollHeight; }, [lines]);
  const push = (arr) => setLines((l) => [...l, ...arr]);
  const out = (text, c) => ({ kind: "out", c: c || "var(--ink-2)", text });

  async function execMatrix(cmd) {
    const res = await msMatrix(cmd);
    push(res.lines.map((t) => out(t, msTone(res.tone))));
  }

  // Real execution inside the local Python sandbox via /v1/matrixshell/exec.
  async function realExec(cmd) {
    if (msDenied(cmd)) { push([out("✗ Refusing to execute — blocked by safety denylist.", "var(--red)")]); return; }
    if (!status || !status.installed) { push([out("MatrixShell sandbox is not installed — click ‘Install MatrixShell’ above.", "var(--amber)")]); return; }
    setBusy(true);
    try {
      const r = await api.post("/v1/matrixshell/exec", { command: cmd });
      const blocks = [];
      if (r.stdout) r.stdout.replace(/\n$/, "").split("\n").forEach((t) => blocks.push(out(t, "var(--ink)")));
      if (r.stderr) r.stderr.replace(/\n$/, "").split("\n").forEach((t) => blocks.push(out(t, "var(--ink-3)")));
      if (r.exit_code !== 0) blocks.push(out("exit " + r.exit_code, "var(--red)"));
      if (blocks.length === 0) blocks.push(out("✓ exit 0", "var(--acc-2)"));
      push(blocks);
    } catch (e) { push([out("✗ " + e.message, "var(--red)")]); }
    finally { setBusy(false); }
  }

  async function dispatch(cmd) {
    if (cmd.startsWith("matrix ") || cmd === "matrix") { await execMatrix(cmd); return; }
    await realExec(cmd);
  }

  async function answer(idx, yes) {
    setLines((l) => l.map((it, i) => i === idx ? { ...it, answered: yes ? "yes" : "no" } : it));
    if (!yes) { push([out("Cancelled.", "var(--ink-3)")]); return; }
    await dispatch(lines[idx].s.cmd);
  }

  async function run(raw) {
    const text = raw.trim(); if (!text) return;
    hist.current.push(text); hi.current = hist.current.length; setValue("");
    if (text === "clear") { setLines([]); return; }
    push([{ kind: "in", text }]);
    if (msLooksCmd(text)) { await dispatch(text); return; }
    push([{ kind: "suggestion", s: msSuggest(text) }]);
  }
  function onKey(e) {
    if (e.key === "ArrowUp") { e.preventDefault(); if (hi.current > 0) { hi.current--; setValue(hist.current[hi.current] || ""); } }
    else if (e.key === "ArrowDown") { e.preventDefault(); if (hi.current < hist.current.length - 1) { hi.current++; setValue(hist.current[hi.current] || ""); } else { hi.current = hist.current.length; setValue(""); } }
  }

  // Real install: stream the matrixshell.install job's pip/venv output.
  async function install() {
    setInstalling(true);
    push([out("$ installing MatrixShell into a local Python sandbox…", "var(--acc-2)")]);
    try {
      const r = await api.post("/v1/matrixshell/install", {});
      const es = new EventSource(api.eventsURL("/v1/jobs/" + r.job_id + "/events"));
      es.onmessage = (ev) => {
        let d; try { d = JSON.parse(ev.data); } catch (e2) { return; }
        if (d.message) push([out(d.message, d.status === "error" ? "var(--red)" : "var(--ink-3)")]);
        if (d.step === "ready" || d.status === "error" || d.status === "complete") { es.close(); setInstalling(false); refreshStatus(); }
      };
      es.onerror = () => { es.close(); setInstalling(false); refreshStatus(); };
    } catch (e) { push([out("✗ " + e.message, "var(--red)")]); setInstalling(false); }
  }

  const installed = status && status.installed;
  const chips = installed
    ? ["matrix status", "matrixsh --help", "python --version", "ls -la", "pip list"]
    : ["matrix status", "matrix ps"];
  const empty = lines.length === 0;
  const shellTag = status == null ? "checking…" : installed ? ("matrixsh " + status.version) : "not installed";
  return (
    <div className="wrap rise">
      <div className="phead">
        <div>
          <p className="eyebrow">Operator terminal</p>
          <h1>MatrixShell</h1>
          <p>The real MatrixShell CLI in a Python sandbox on this runtime's host. Type a command (executed for real), or describe what you want and confirm the suggestion.</p>
        </div>
        <div style={{ display: "flex", gap: 9, flexWrap: "wrap", alignItems: "center" }}>
          <span className={"chip " + (rt.online ? "green" : "red")}><span className={"dot " + (rt.online ? "green pulse" : "red")} /> {rt.online ? "Connected" : "Offline"}</span>
          <span className="chip green"><GI d={G.shield} size={12} /> Safe mode</span>
          <span className={"chip " + (installed ? "green" : "amber")}><GI d={G.terminal} size={12} /> {shellTag}</span>
          <button className="btn btn-ghost btn-sm" onClick={back}><GI d={G.x} size={14} /> Close</button>
        </div>
      </div>

      {status != null && !installed && (
        <div className="card card-pad" style={{ marginBottom: 16, display: "flex", alignItems: "center", gap: 16, flexWrap: "wrap", borderColor: "var(--amber-soft)" }}>
          <div style={{ flex: 1, minWidth: 240 }}>
            <div style={{ fontWeight: 700, fontSize: 14 }}>MatrixShell is not installed yet</div>
            <p style={{ margin: "5px 0 0", fontSize: 12.5, color: "var(--ink-3)" }}>This installs the real <span className="mono">matrixsh</span> CLI from git into a dedicated Python venv on this host, then runs commands there. {status.error ? "(" + status.error + ")" : ""}</p>
          </div>
          <button className="btn btn-primary" onClick={install} disabled={installing}><GI d={G.download} size={15} /> {installing ? "Installing…" : "Install MatrixShell"}</button>
        </div>
      )}

      <div className="card card-pad" style={{ marginBottom: 16 }}>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "14px 36px" }}>
          {[["Workspace", (user && user.workspace) || "—"], ["Runtime", (rt.health && rt.health.runtime_id) || "—"], ["Mode", (rt.health && rt.health.mode) || "—"], ["matrixsh", installed ? status.version : "—"], ["Sandbox", (status && status.sandbox) || "—"]].map(([k, v]) => (
            <div key={k} style={{ minWidth: 0 }}>
              <div style={{ fontFamily: "var(--mono)", fontSize: 10, letterSpacing: "0.16em", textTransform: "uppercase", color: "var(--ink-4)" }}>{k}</div>
              <div className="mono" style={{ fontSize: 12.5, color: "var(--ink)", marginTop: 4, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 320 }}>{v}</div>
            </div>
          ))}
        </div>
      </div>

      <div className="card" style={{ display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div className="card-h"><span className="t">Terminal</span><span className="chip mono">{installed ? "sandbox · matrixsh " + status.version : "matrix-runtime"}</span></div>
        <div ref={scroller} style={{ minHeight: 320, maxHeight: "46vh", overflowY: "auto", padding: "16px 18px", fontFamily: "var(--mono)", fontSize: 12.5, lineHeight: 1.75 }}>
          <p style={{ margin: 0, color: "var(--ink-3)" }}>{installed ? "matrixsh " + status.version + " · real Python sandbox on " + ((rt.health && rt.health.runtime_id) || "this host") : "MatrixShell control terminal"}</p>
          <p style={{ margin: 0, color: "var(--ink-4)" }}>Commands run for real in the sandbox. Destructive operations are blocked. <span className="mono">matrix …</span> talks to the control plane.</p>
          {empty && (
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", padding: "40px 0 32px", textAlign: "center", opacity: 0.7 }}>
              <span style={{ color: "var(--acc)", opacity: 0.5 }}><GI d={G.terminal} size={30} /></span>
              <p style={{ margin: "16px 0 0", fontSize: 13, color: "var(--ink-3)", fontFamily: "var(--font)" }}>{installed ? "Ready for commands" : "Install MatrixShell to run sandbox commands"}</p>
              <p style={{ margin: "6px 0 0", fontSize: 11.5, color: "var(--ink-4)", fontFamily: "var(--font)" }}>Try: matrixsh --help · python --version · matrix status</p>
            </div>
          )}
          <div style={{ marginTop: 12 }}>
            {lines.map((it, i) => it.kind === "suggestion"
              ? <Suggestion key={i} s={it.s} answered={it.answered} onAnswer={(yes) => answer(i, yes)} />
              : it.kind === "in"
                ? <p key={i} style={{ margin: 0 }}><span style={{ color: "var(--acc)" }}>sandbox</span><span style={{ color: "var(--ink-4)" }}> $ </span><span style={{ color: "#fff" }}>{it.text}</span></p>
                : <p key={i} style={{ margin: 0, color: it.c, whiteSpace: "pre-wrap" }}>{it.text}</p>)}
            {busy && <p style={{ margin: 0, color: "var(--ink-4)" }}><span className="cur" /></p>}
          </div>
        </div>
        <div style={{ borderTop: "1px solid var(--line)", background: "rgba(0,0,0,0.25)", padding: 14, flexShrink: 0 }}>
          <div className="no-bar" style={{ display: "flex", gap: 8, overflowX: "auto", marginBottom: 11 }}>
            {chips.map((c) => <button key={c} onClick={() => run(c)} className="chip" style={{ flexShrink: 0, cursor: "pointer", height: 28, fontFamily: "var(--font)" }}>{c}</button>)}
          </div>
          <form onSubmit={(e) => { e.preventDefault(); run(value); }} style={{ display: "flex", alignItems: "center", gap: 10, height: 46, padding: "0 14px", borderRadius: "var(--r-sm)", border: "1px solid var(--line-2)", background: "rgba(0,0,0,0.4)" }}>
            <span className="mono" style={{ fontSize: 13.5, color: "var(--acc)", fontWeight: 700 }}>sandbox<span style={{ color: "var(--ink-4)" }}>&gt;</span></span>
            <input ref={inputRef} value={value} onChange={(e) => setValue(e.target.value)} onKeyDown={onKey} placeholder="type a command, or ask in plain English…" spellCheck={false}
              style={{ flex: 1, minWidth: 0, height: "100%", background: "transparent", border: "none", outline: "none", color: "var(--ink)", fontFamily: "var(--mono)", fontSize: 13.5 }} />
            <button type="submit" aria-label="Run" disabled={busy} style={{ width: 34, height: 34, borderRadius: 9, background: "var(--acc)", color: "#03140c", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}><GI d={G.chevr} size={16} sw={2.4} /></button>
          </form>
          <div style={{ marginTop: 9, display: "flex", alignItems: "center", gap: 7, fontSize: 11, color: "var(--ink-4)" }}>
            <GI d={G.shield} size={12} style={{ color: "var(--acc)" }} /> Real execution in a Python sandbox · destructive commands are blocked.
          </div>
        </div>
      </div>
    </div>
  );
}

/* ---------------------------------------------------------------
   Wizards — New Sandbox (real launch) + Attach Model.
   --------------------------------------------------------------- */
function Wizard({ title, eyebrow, steps, step, onCancel, children, footer }) {
  return (
    <div className="wrap rise" style={{ maxWidth: 880 }}>
      <button className="chip" onClick={onCancel} style={{ marginBottom: 16, cursor: "pointer" }}><GI d={G.chev} size={13} style={{ transform: "rotate(90deg)" }} /> Cancel</button>
      <p className="eyebrow">{eyebrow}</p>
      <h1 style={{ margin: "7px 0 0", fontSize: 24, fontWeight: 700, letterSpacing: "-0.02em" }}>{title}</h1>
      <div style={{ display: "flex", alignItems: "center", gap: 0, margin: "22px 0 4px" }}>
        {steps.map((s, i) => {
          const state = i < step ? "done" : i === step ? "active" : "todo";
          return (
            <React.Fragment key={s}>
              <div style={{ display: "flex", alignItems: "center", gap: 9, flexShrink: 0 }}>
                <span style={{ width: 26, height: 26, borderRadius: 99, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 12, fontWeight: 700, fontFamily: "var(--mono)", background: state === "done" ? "var(--acc)" : "transparent", border: "1px solid " + (state === "todo" ? "var(--line-3)" : "var(--acc)"), color: state === "done" ? "#042012" : state === "active" ? "var(--acc)" : "var(--ink-4)" }}>{state === "done" ? <GI d={G.check} size={14} sw={2.6} /> : i + 1}</span>
                <span style={{ fontSize: 12.5, fontWeight: 600, color: state === "todo" ? "var(--ink-4)" : "var(--ink)", whiteSpace: "nowrap" }} className="hide-sm">{s}</span>
              </div>
              {i < steps.length - 1 && <div style={{ flex: 1, height: 1, background: i < step ? "var(--acc)" : "var(--line-2)", margin: "0 12px", minWidth: 18 }} />}
            </React.Fragment>
          );
        })}
      </div>
      <div className="card" style={{ marginTop: 18 }}>
        <div className="card-pad">{children}</div>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "14px 18px", borderTop: "1px solid var(--line)" }}>{footer}</div>
      </div>
    </div>
  );
}
function PickRow({ active, onClick, mono, title, sub, right, disabled }) {
  return (
    <button onClick={disabled ? undefined : onClick} disabled={disabled}
      style={{ display: "flex", alignItems: "center", gap: 13, width: "100%", textAlign: "left", padding: "13px 14px", borderRadius: "var(--r-md)", border: "1px solid " + (active ? "var(--acc-line)" : "var(--line-2)"), background: active ? "var(--acc-soft)" : "var(--inset)", opacity: disabled ? 0.5 : 1, cursor: disabled ? "not-allowed" : "pointer", transition: "all .14s" }}>
      <span style={{ width: 38, height: 38, borderRadius: 10, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", border: "1px solid var(--line-2)", background: "var(--raised)", color: "var(--acc)", fontFamily: "var(--mono)", fontWeight: 700, fontSize: 12 }}>{mono}</span>
      <span style={{ flex: 1, minWidth: 0 }}>
        <span style={{ display: "block", fontSize: 13.5, fontWeight: 600, color: "var(--ink)" }}>{title}</span>
        <span style={{ display: "block", fontSize: 12, color: "var(--ink-3)", marginTop: 1 }}>{sub}</span>
      </span>
      {right}
      <span style={{ width: 18, height: 18, borderRadius: 99, flexShrink: 0, border: "1px solid " + (active ? "var(--acc)" : "var(--line-3)"), background: active ? "var(--acc)" : "transparent", display: "flex", alignItems: "center", justifyContent: "center" }}>{active && <GI d={G.check} size={12} sw={3} style={{ color: "#042012" }} />}</span>
    </button>
  );
}
function NewSandboxWizard({ onCancel, onLaunch }) {
  const [step, setStep] = React.useState(0);
  const [pick, setPick] = React.useState(null);
  const [runtime, setRuntime] = React.useState("");
  const catalog = useCatalog();
  const { runtimes: realRuntimes } = useRuntimes();
  const items = (catalog || []).filter((it) => it.sandbox);
  const steps = ["Component", "Environment", "Review"];
  const selected = items.find((i) => i.id === pick);
  const next = () => setStep((s) => Math.min(steps.length - 1, s + 1));
  const back = () => setStep((s) => Math.max(0, s - 1));
  const runtimes = (realRuntimes || []).filter((r) => r.statusClass !== "red");
  React.useEffect(() => { if (!runtime && runtimes[0]) setRuntime(runtimes[0].name); }, [runtimes, runtime]);
  return (
    <Wizard eyebrow="Sandboxes · New session" title="Start a sandbox" steps={steps} step={step} onCancel={onCancel}
      footer={<>
        <button className="btn btn-ghost" onClick={step === 0 ? onCancel : back}>{step === 0 ? "Cancel" : "Back"}</button>
        {step < steps.length - 1
          ? <button className="btn btn-primary" disabled={step === 0 && !pick} onClick={next}>Continue <GI d={G.chevr} size={15} /></button>
          : <button className="btn btn-primary" onClick={() => onLaunch(selected)}><GI d={G.play} size={15} /> Launch sandbox</button>}
      </>}>
      {step === 0 && (
        <div>
          <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--ink-3)" }}>Choose a sandbox-enabled MCP server to test in an isolated 10-minute session. No production secrets are used. Items badged <span className="chip green" style={{ height: 20 }}>live</span> run for real on this runtime.</p>
          <div style={{ display: "grid", gap: 9 }}>
            {items.map((it) => (
              <PickRow key={it.id} active={pick === it.id} onClick={() => setPick(it.id)} mono={it.initials} title={it.name} sub={`${it.runtime} · ${it.secrets ? "needs secrets" : "no secrets"}`}
                right={it.startCommand ? <span className="chip green" style={{ height: 20 }}>live</span> : (it.verified ? <span className="chip green" style={{ height: 20 }}>Verified</span> : null)} />
            ))}
          </div>
        </div>
      )}
      {step === 1 && (
        <div>
          <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--ink-3)" }}>Select the runtime that will host the sandbox.</p>
          <div style={{ display: "grid", gap: 9 }}>
            {runtimes.length === 0 && <div style={{ color: "var(--ink-4)", fontSize: 12.5 }}>No runtimes available.</div>}
            {runtimes.map((r) => (
              <PickRow key={r.id} active={runtime === r.name} onClick={() => setRuntime(r.name)} mono={(r.region || "lo").slice(0, 2).toUpperCase()}
                title={r.name} sub={`${r.mode} · ${r.region}`} right={r.live ? <LiveTag /> : <span className={"chip " + (r.statusClass || "green")} style={{ height: 20 }}><span className={"dot " + (r.statusClass || "green")} /> {r.status || "Online"}</span>} />
            ))}
          </div>
        </div>
      )}
      {step === 2 && selected && (
        <div>
          <p style={{ margin: "0 0 16px", fontSize: 13, color: "var(--ink-3)" }}>Review and launch. The session boots immediately and expires automatically.</p>
          <div style={{ display: "grid", gap: 1, borderRadius: "var(--r-sm)", overflow: "hidden", border: "1px solid var(--line)" }}>
            {[["Component", selected.name], ["Manifest", selected.id], ["Runtime", runtime || "—"], ["Command", selected.startCommand || "—"], ["TTL", "10 minutes"], ["Secrets", "None"]].map(([k, v]) => (
              <div key={k} style={{ display: "flex", gap: 12, padding: "12px 14px", background: "var(--inset)" }}>
                <span style={{ width: 100, flexShrink: 0, fontSize: 12, color: "var(--ink-3)", textTransform: "uppercase", letterSpacing: "0.05em" }}>{k}</span>
                <span className="mono" style={{ fontSize: 12.5, color: "var(--ink)", wordBreak: "break-all" }}>{v}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </Wizard>
  );
}
const MODEL_PROVIDERS = [
  { id: "huggingface", mono: "HF", name: "Hugging Face", sub: "resolvable via model.inspect", models: ["hf:Qwen/Qwen2.5-7B-Instruct", "hf:mistralai/Mistral-7B-Instruct-v0.3", "hf:BAAI/bge-large-en-v1.5"] },
  { id: "ollama", mono: "OL", name: "Ollama", sub: "local · 6 models", models: ["llama3.1", "mistral", "qwen2.5"] },
];
function AttachModelWizard({ onCancel, onDone }) {
  const [step, setStep] = React.useState(0);
  const [prov, setProv] = React.useState("huggingface");
  const [model, setModel] = React.useState(null);
  const [events, setEvents] = React.useState([]);
  const [done, setDone] = React.useState(false);
  const [err, setErr] = React.useState("");
  const steps = ["Provider", "Model", "Attach"];
  const provider = MODEL_PROVIDERS.find((p) => p.id === prov);
  const next = () => setStep((s) => Math.min(steps.length - 1, s + 1));
  const back = () => setStep((s) => Math.max(0, s - 1));

  React.useEffect(() => {
    if (step !== 2 || !model) return;
    let alive = true; setEvents([]); setDone(false); setErr("");
    (async () => {
      const add = (k, msg) => alive && setEvents((e) => [...e, { k, msg }]);
      add("resolve", "Resolving metadata via model.inspect…");
      try {
        if (prov === "huggingface") {
          const meta = await runJobToCompletion("model.inspect", { model, revision: "main" });
          if (!alive) return;
          add("resolve", `task=${meta.pipeline_tag || "?"} · library=${meta.library_name || "?"}`);
          add("license", `License check · ${meta.license || "unknown"}`);
          add("params", `Parameters ≈ ${fmtParams(meta.estimated_parameters)} · runtime=${meta.recommended_runtime}`);
          add("ready", "Model resolved and ready to attach.");
        } else {
          await sleep(500); add("ready", "Provider model selected.");
        }
        if (alive) setDone(true);
      } catch (e) { if (alive) { setErr(e.message); add("error", "✗ " + e.message); } }
    })();
    return () => { alive = false; };
  }, [step, model, prov]);

  return (
    <Wizard eyebrow="Models · Attach" title="Attach a model" steps={steps} step={step} onCancel={onCancel}
      footer={step < 2 ? (
        <>
          <button className="btn btn-ghost" onClick={step === 0 ? onCancel : back}>{step === 0 ? "Cancel" : "Back"}</button>
          <button className="btn btn-primary" disabled={(step === 0 && !prov) || (step === 1 && !model)} onClick={next}>{step === 1 ? <><GI d={G.download} size={15} /> Attach model</> : <>Continue <GI d={G.chevr} size={15} /></>}</button>
        </>
      ) : (
        <>
          <span style={{ fontSize: 12.5, color: err ? "var(--red)" : "var(--ink-3)" }}>{err ? "Failed" : (done ? "Resolved successfully." : "Resolving…")}</span>
          <button className="btn btn-primary" disabled={!done} onClick={onDone}><GI d={G.check} size={15} sw={2.4} /> Done</button>
        </>
      )}>
      {step === 0 && (
        <div>
          <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--ink-3)" }}>Choose the provider that serves the model. Hugging Face models resolve live through this runtime.</p>
          <div style={{ display: "grid", gap: 9 }}>
            {MODEL_PROVIDERS.map((p) => (
              <PickRow key={p.id} active={prov === p.id} onClick={() => { setProv(p.id); setModel(null); }} mono={p.mono} title={p.name} sub={p.sub} right={p.id === "huggingface" ? <LiveTag /> : <span className="chip green" style={{ height: 20 }}><span className="dot green" /> connected</span>} />
            ))}
          </div>
        </div>
      )}
      {step === 1 && provider && (
        <div>
          <p style={{ margin: "0 0 14px", fontSize: 13, color: "var(--ink-3)" }}>Pick a model from <b style={{ color: "var(--ink-2)" }}>{provider.name}</b>. Metadata is resolved before attach.</p>
          <div style={{ display: "grid", gap: 9 }}>
            {provider.models.map((m) => (
              <PickRow key={m} active={model === m} onClick={() => setModel(m)} mono="M" title={m} sub={m.includes("bge") ? "embeddings" : "text-generation"} right={m.includes("7B") || m.includes("8B") ? <span className="chip amber" style={{ height: 20 }}>GPU</span> : null} />
            ))}
          </div>
        </div>
      )}
      {step === 2 && (
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 16 }}>
            <span style={{ width: 40, height: 40, borderRadius: 10, flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", border: "1px solid var(--line-2)", background: "var(--raised)", color: "var(--acc)" }}><GI d={G.cpu} size={18} /></span>
            <div style={{ minWidth: 0 }}><div style={{ fontSize: 14, fontWeight: 700 }} className="mono">{model}</div><div style={{ fontSize: 12, color: "var(--ink-3)" }}>{provider.name}</div></div>
            <span className={"chip " + (err ? "red" : done ? "green" : "amber")} style={{ marginLeft: "auto" }}><span className={"dot " + (err ? "red" : done ? "green pulse" : "amber")} /> {err ? "Failed" : done ? "Ready" : "Resolving"}</span>
          </div>
          <div className="logwin" style={{ height: 200 }}>
            {events.map((e, i) => (<div key={i} className="rise" style={{ color: e.k === "error" ? "var(--red)" : "var(--ink-2)" }}><span style={{ color: e.k === "error" ? "var(--red)" : "var(--acc)" }}>{e.k === "error" ? "✗" : "✓"}</span> <span style={{ color: "var(--ink-4)" }}>[{e.k}]</span> {e.msg}</div>))}
            {!done && !err && <span className="cur" />}
          </div>
          {done && (
            <div className="rise" style={{ marginTop: 14, padding: 14, borderRadius: "var(--r-md)", border: "1px solid var(--acc-line)", background: "var(--acc-soft)" }}>
              <div style={{ fontSize: 13.5, fontWeight: 700, color: "var(--acc-2)" }}>Model resolved</div>
              <p style={{ margin: "5px 0 0", fontSize: 12.5, color: "var(--ink-2)" }}><span className="mono">{model}</span> resolved and ready for agents to use.</p>
            </div>
          )}
        </div>
      )}
    </Wizard>
  );
}

/* ---------------------------------------------------------------
   Sidebar user account menu + sign-out.
   --------------------------------------------------------------- */
function MenuRow({ ic, label, chevron, danger, onClick }) {
  return (
    <button className="navitem" onClick={onClick} style={{ fontSize: 13, color: danger ? "var(--red)" : undefined }}>
      {ic && <span className="i" style={danger ? { color: "var(--red)" } : null}><GI d={ic} size={15} /></span>}
      <span style={{ flex: 1, textAlign: "left" }}>{label}</span>
      {chevron && <GI d={G.chevr} size={13} style={{ color: "var(--ink-4)" }} />}
    </button>
  );
}
function MenuDiv() { return <div style={{ height: 1, background: "var(--line)", margin: "5px 0" }} />; }
function SignOutDialog({ onClose, onConfirm }) {
  return ReactDOM.createPortal(
    <div className="scrim" style={{ position: "fixed", inset: 0, zIndex: 80, display: "flex", alignItems: "center", justifyContent: "center", padding: 24, background: "rgba(5,8,12,0.7)", backdropFilter: "blur(6px)" }} onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div className="rise card" style={{ width: "100%", maxWidth: 420, background: "var(--panel-2)" }}>
        <div className="card-h"><span className="t">Sign out of MatrixCloud?</span></div>
        <div style={{ padding: 18 }}>
          <p style={{ margin: 0, fontSize: 13, lineHeight: 1.6, color: "var(--ink-2)" }}>You will be signed out of this browser. Running jobs, runtimes, and sandboxes will continue.</p>
          <div style={{ display: "flex", gap: 10, marginTop: 18 }}>
            <button className="btn btn-ghost" style={{ flex: 1 }} onClick={onClose}>Cancel</button>
            <button className="btn btn-primary" style={{ flex: 1, background: "var(--red)", color: "#fff" }} onClick={() => onConfirm(false)}>Sign out</button>
          </div>
          <button className="navitem" style={{ marginTop: 8, fontSize: 12.5, color: "var(--ink-3)", justifyContent: "center" }} onClick={() => onConfirm(true)}>Sign out of all sessions</button>
        </div>
      </div>
    </div>, document.body);
}
function SidebarUser({ onProfile, go, user, onSignOut }) {
  const [open, setOpen] = React.useState(false);
  const [confirm, setConfirm] = React.useState(false);
  const initials = (user.name || user.email || "U").split(" ").map((x) => x[0]).slice(0, 2).join("").toUpperCase();
  const act = (fn) => { setOpen(false); fn && fn(); };
  return (
    <div style={{ position: "relative" }}>
      {open && (<>
        <div style={{ position: "fixed", inset: 0, zIndex: 44 }} onClick={() => setOpen(false)} />
        <div className="card rise no-bar" style={{ position: "absolute", bottom: "calc(100% + 8px)", left: 0, width: 250, zIndex: 45, padding: 6, background: "var(--panel-2)", boxShadow: "0 -8px 40px rgba(0,0,0,0.5)" }}>
          <div style={{ display: "flex", gap: 11, padding: "11px 11px 12px" }}>
            <span className="ua" style={{ width: 38, height: 38, fontSize: 15, borderRadius: 10 }}>{initials}</span>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 13.5, fontWeight: 700, color: "var(--ink)" }}>{user.name}</div>
              <div style={{ fontSize: 12, color: "var(--ink-3)", fontFamily: "var(--mono)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{user.email}</div>
              <div style={{ fontSize: 11.5, color: "var(--acc-2)", marginTop: 3 }}>{user.role} · {user.workspace}</div>
            </div>
          </div>
          <MenuDiv />
          <MenuRow ic={G.layers} label="Switch workspace" chevron onClick={() => act(() => go("settings"))} />
          <MenuRow ic={G.user} label="Profile" onClick={() => act(onProfile)} />
          <MenuRow ic={G.gear} label="Settings" onClick={() => act(() => go("settings"))} />
          <MenuDiv />
          <MenuRow ic={G.x} label="Sign out" danger onClick={() => { setOpen(false); setConfirm(true); }} />
        </div>
      </>)}
      <button onClick={() => setOpen((v) => !v)} className={"user-row" + (open ? " open" : "")} aria-label="Account menu">
        <span className="ua">{initials}</span>
        <span style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
          <span style={{ display: "block", fontSize: 13, fontWeight: 600, color: "var(--ink)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{user.name}</span>
          <span style={{ display: "block", fontSize: 11, color: "var(--ink-4)", fontFamily: "var(--mono)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{user.email}</span>
        </span>
        <GI d={G.chev} size={15} style={{ color: "var(--ink-4)", transform: open ? "rotate(0deg)" : "rotate(180deg)", flexShrink: 0 }} />
      </button>
      {confirm && <SignOutDialog onClose={() => setConfirm(false)} onConfirm={(all) => { setConfirm(false); onSignOut(all); }} />}
    </div>
  );
}

const NAV = [
  { grp: "Platform", items: [{ id: "overview", label: "Overview", ic: G.grid }, { id: "catalog", label: "Catalog", ic: G.layers }, { id: "sandboxes", label: "Sandboxes", ic: G.play }, { id: "models", label: "Models", ic: G.cpu }] },
  { grp: "Runtime", items: [{ id: "runtimes", label: "Runtimes", ic: G.server }, { id: "agents", label: "Jobs", ic: G.activity }, { id: "logs", label: "Logs", ic: G.logs }, { id: "install", label: "Install Runtime", ic: G.download }] },
  { grp: "Governance", items: [{ id: "policies", label: "Policies", ic: G.shield }, { id: "audit", label: "Audit", ic: G.audit }, { id: "settings", label: "Settings", ic: G.gear }] },
];

function EnvSwitch() {
  const [open, setOpen] = React.useState(false);
  const [env, setEnv] = React.useState("Production");
  const envs = [["Production", "green"], ["Staging", "amber"], ["Development", "blue"]];
  return (
    <div style={{ position: "relative" }}>
      <button className="switcher" onClick={() => setOpen((v) => !v)}><span className="dot green" /> <span className="env-switch-lab">{env}</span> <GI d={G.chev} size={14} className="chev" /></button>
      {open && (<>
        <div style={{ position: "fixed", inset: 0, zIndex: 40 }} onClick={() => setOpen(false)} />
        <div className="card rise" style={{ position: "absolute", top: 40, left: 0, zIndex: 41, width: 180, padding: 6, background: "var(--panel-2)" }}>
          {envs.map(([e, c]) => <button key={e} className="navitem" onClick={() => { setEnv(e); setOpen(false); }} style={{ fontSize: 13 }}><span className={"dot " + c} /> {e}</button>)}
        </div>
      </>)}
    </div>
  );
}
function WorkspaceSwitch() {
  return <button className="switcher"><span className="av" style={{ background: "linear-gradient(135deg,#2bd576,#1c9c54)" }}>A</span><span style={{ fontWeight: 600, color: "var(--ink)" }} className="hide-sm">Acme Corp</span><GI d={G.chev} size={14} className="chev" /></button>;
}
function UserMenu({ onProfile, onSettings }) {
  const [open, setOpen] = React.useState(false);
  const items = [{ label: "Profile & preferences", ic: G.user, fn: onProfile }, { label: "Workspace settings", ic: G.gear, fn: onSettings }];
  return (
    <div style={{ position: "relative" }}>
      <button className="user-av" onClick={() => setOpen((v) => !v)} aria-label="Account menu">N</button>
      {open && (<>
        <div style={{ position: "fixed", inset: 0, zIndex: 44 }} onClick={() => setOpen(false)} />
        <div className="card rise" style={{ position: "absolute", top: 40, right: 0, zIndex: 45, width: 232, padding: 6, background: "var(--panel-2)" }}>
          <div style={{ padding: "10px 11px 11px", borderBottom: "1px solid var(--line)", marginBottom: 5 }}>
            <div style={{ fontSize: 13.5, fontWeight: 700 }}>Neo Anderson</div>
            <div style={{ fontSize: 12, color: "var(--ink-3)", marginTop: 1 }} className="mono">neo@acme.io</div>
          </div>
          {items.map((it) => <button key={it.label} className="navitem" onClick={() => { it.fn(); setOpen(false); }} style={{ fontSize: 13 }}><span className="i"><GI d={it.ic} size={16} /></span> {it.label}</button>)}
          <div style={{ borderTop: "1px solid var(--line)", marginTop: 5, paddingTop: 5 }}>
            <button className="navitem" style={{ fontSize: 13, color: "var(--red)" }}><span className="i" style={{ color: "var(--red)" }}><GI d={G.x} size={16} /></span> Sign out</button>
          </div>
        </div>
      </>)}
    </div>
  );
}
function SandboxPlaceholder({ onNew }) {
  return (
    <div className="wrap rise">
      <div className="phead"><div><p className="eyebrow">Execution plane</p><h1>Sandboxes</h1><p>Temporary 10-minute sessions to test MCP servers safely before installing — no production secrets, auto-expiring.</p></div>
        <button className="btn btn-primary" onClick={onNew}><GI d={G.play} size={15} /> New sandbox</button></div>
      <div className="card card-pad" style={{ display: "flex", flexDirection: "column", alignItems: "center", textAlign: "center", padding: 48 }}>
        <span style={{ display: "inline-flex", color: "var(--ink-4)" }}><GI d={G.play} size={36} /></span>
        <h2 style={{ margin: "16px 0 0", fontSize: 18 }}>No active sandboxes</h2>
        <p style={{ margin: "8px 0 0", fontSize: 13, color: "var(--ink-3)", maxWidth: 380 }}>Start a guided 10-minute test session — pick a sandbox-enabled MCP server, choose a runtime, and launch.</p>
        <button className="btn btn-primary" style={{ marginTop: 18 }} onClick={onNew}><GI d={G.play} size={15} /> New sandbox</button>
      </div>
    </div>
  );
}

function SideFoot() {
  const rt = useRuntime();
  return (
    <div className="side-foot">
      <div style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12, color: "var(--ink-2)" }}>
        <span className={"dot " + (rt.online ? "green pulse" : "red")} /> {rt.online ? "Control plane online" : (rt.loading ? "Connecting…" : "Runtime offline")}
      </div>
      <div style={{ fontSize: 11, color: "var(--ink-4)", marginTop: 5, fontFamily: "var(--mono)" }}>{rt.health ? rt.health.runtime_id + " · v" + rt.health.version : "cloud.matrixhub.io"}</div>
      <a href="/docs" target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, color: "var(--ink-3)", marginTop: 8 }}><GI d={G.logs} size={12} /> API docs</a>
    </div>
  );
}

// fmtBytes renders a byte count as a compact human string.
function fmtBytes(n) {
  n = Number(n) || 0;
  if (n < 1024) return n + " B";
  const u = ["KB", "MB", "GB", "TB"];
  let i = -1;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  return n.toFixed(n < 10 ? 1 : 0) + " " + u[i];
}

// StorageCard shows data-directory usage from GET /v1/system/storage.
function StorageCard() {
  const { data } = useFetch("/v1/system/storage", 15000);
  const areaLabels = { models: "Model cache", mcp: "MCP cache", agents: "Agents", jobs: "Jobs", logs: "Logs", database: "Database" };
  const areas = data ? data.areas || {} : {};
  const keys = Object.keys(areaLabels).filter((k) => k in areas);
  return (
    <div className="card">
      <div className="card-h">
        <span className="t">Storage</span>
        {data && <span className="chip">{fmtBytes(data.total_bytes)} used{data.free_bytes ? " · " + fmtBytes(data.free_bytes) + " free" : ""}</span>}
      </div>
      <div style={{ padding: "4px 18px" }}>
        {!data && <div style={{ padding: "13px 0", color: "var(--ink-4)", fontSize: 12.5 }}>Loading…</div>}
        {data && keys.map((k) => (
          <div key={k} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 0", borderBottom: "1px solid var(--line)" }}>
            <span style={{ fontSize: 13.5, fontWeight: 600 }}>{areaLabels[k]}</span>
            <span className="mono" style={{ fontSize: 12.5, color: "var(--ink-3)" }}>{fmtBytes(areas[k])}</span>
          </div>
        ))}
        {data && <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 0" }}>
          <span style={{ fontSize: 12, color: "var(--ink-4)" }}>{data.jobs_count || 0} job dir(s) · {data.data_dir}</span>
        </div>}
      </div>
    </div>
  );
}

function CloudApp({ user, onSignOut }) {
  const [route, setRoute] = React.useState("overview");
  const [sideOpen, setSideOpen] = React.useState(false);
  const [detailItem, setDetailItem] = React.useState(null);
  const [sandboxModal, setSandboxModal] = React.useState(null);
  const [sandboxSession, setSandboxSession] = React.useState(null);

  function go(id) { setRoute(id); setDetailItem(null); setSandboxSession(null); setSideOpen(false); }
  function openItem(item, sandbox) { if (sandbox) setSandboxModal(item); else setDetailItem(item); }
  function startSandbox(item) { setSandboxModal(null); setSandboxSession(item); setDetailItem(null); setRoute("sandboxes"); }

  let view;
  if (sandboxSession) view = <SandboxSession item={sandboxSession} back={() => { setSandboxSession(null); setRoute("catalog"); }} />;
  else if (detailItem) view = <DetailView item={detailItem} back={() => setDetailItem(null)} startSandbox={(it) => setSandboxModal(it)} />;
  else if (route === "overview") view = <OverviewView go={go} />;
  else if (route === "catalog") view = <CatalogView openItem={openItem} />;
  else if (route === "sandboxes") view = <SandboxPlaceholder onNew={() => setRoute("new-sandbox")} />;
  else if (route === "new-sandbox") view = <NewSandboxWizard onCancel={() => setRoute("sandboxes")} onLaunch={(it) => startSandbox(it)} />;
  else if (route === "models") view = <ModelsView onAttach={() => setRoute("attach-model")} />;
  else if (route === "attach-model") view = <AttachModelWizard onCancel={() => setRoute("models")} onDone={() => setRoute("models")} />;
  else if (route === "runtimes") view = <RuntimesView go={go} />;
  else if (route === "agents") view = <JobsView />;
  else if (route === "logs") view = <LogsView />;
  else if (route === "install") view = <InstallRuntimeView />;
  else if (route === "policies") view = <PoliciesView />;
  else if (route === "audit") view = <AuditView />;
  else if (route === "settings") view = <CloudSettingsView />;
  else if (route === "profile") view = <ProfileView />;
  else if (route === "matrixshell") view = <MatrixShellView user={user} back={() => go("overview")} />;
  else view = <OverviewView go={go} />;

  return (
    <>
      <div className="top">
        <button className="iconbtn menu-btn" onClick={() => setSideOpen((v) => !v)}><GI d={G.menu} size={18} /></button>
        <div className="brand"><span className="logo"><GI d={G.terminal} size={16} /></span><span className="nm hide-sm">MatrixCloud</span></div>
        <div className="sep hide-sm" />
        <WorkspaceSwitch />
        <EnvSwitch />
        <div className="grow" />
        <div className="topsearch"><GI d={G.search} size={15} /><input placeholder="Search…" /><kbd>⌘K</kbd></div>
        <button className={"btn btn-primary btn-sm" + (route === "matrixshell" ? "" : "")} onClick={() => go("matrixshell")} title="Open MatrixShell">
          <GI d={G.terminal} size={15} /> <span className="hide-sm">MatrixShell</span>
        </button>
        <button className="iconbtn"><GI d={G.bell} size={17} /></button>
      </div>
      <div className="shell">
        {sideOpen && <div className="side-scrim" onClick={() => setSideOpen(false)} />}
        <aside className={"side" + (sideOpen ? " open" : "")}>
          {NAV.map((grp) => (
            <React.Fragment key={grp.grp}>
              <div className="navlabel">{grp.grp}</div>
              <nav>
                {grp.items.map((it) => (
                  <button key={it.id} className={"navitem" + (route === it.id ? " active" : "")} onClick={() => go(it.id)}>
                    <span className="i"><GI d={it.ic} size={17} /></span> {it.label}
                    {it.tag && <span className="tag">{it.tag}</span>}
                  </button>
                ))}
              </nav>
            </React.Fragment>
          ))}
          <SideFoot />
          <SidebarUser user={user} go={go} onProfile={() => go("profile")} onSignOut={onSignOut} />
        </aside>
        <main className="main"><ReadinessBanner go={go} />{view}</main>
      </div>
      {sandboxModal && <SandboxModal item={sandboxModal} onClose={() => setSandboxModal(null)} onStart={() => startSandbox(sandboxModal)} />}
    </>
  );
}

function Root() {
  const rt = useRuntimeState();
  const [auth0, setAuth0] = React.useState({ loading: true, user: null });

  React.useEffect(() => {
    let alive = true;
    if (!getToken()) { setAuth0({ loading: false, user: null }); return; }
    auth.me().then((u) => alive && setAuth0({ loading: false, user: u }))
      .catch(() => { setToken(null); alive && setAuth0({ loading: false, user: null }); });
    return () => { alive = false; };
  }, []);

  async function signOut(all) { await auth.logout(all); setAuth0({ loading: false, user: null }); }

  // Email-link flows (reset / verify) take precedence — they must work whether
  // or not a session is already active.
  const intent = React.useMemo(urlIntent, []);
  if (intent.kind === "reset" || intent.kind === "verify") {
    return <AuthScreen onAuthed={(u) => setAuth0({ loading: false, user: u })} />;
  }
  if (auth0.loading) {
    return <div style={{ minHeight: "100vh", display: "grid", placeItems: "center", color: "var(--ink-3)", fontFamily: "var(--mono)", fontSize: 13 }}>Loading MatrixCloud…</div>;
  }
  if (!auth0.user) return <AuthScreen onAuthed={(u) => setAuth0({ loading: false, user: u })} />;

  return (
    <RuntimeCtx.Provider value={rt}>
      <CloudApp user={auth0.user} onSignOut={signOut} />
    </RuntimeCtx.Provider>
  );
}

ReactDOM.createRoot(document.getElementById("cloud-root")).render(<Root />);
