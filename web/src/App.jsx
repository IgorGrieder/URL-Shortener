import { useEffect, useMemo, useState } from "react";
import {
  checkHealth,
  createLink,
  fetchStats,
  loadSettings,
  saveSettings,
  sanitizeBaseUrl
} from "./api";

const MS_PER_DAY = 24 * 60 * 60 * 1000;

function toDateInput(date) {
  return date.toISOString().slice(0, 10);
}

function toISODateTime(value) {
  if (!value) return null;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return null;
  return date.toISOString();
}

function formatMaybe(value) {
  if (!value) return "-";
  return value.replace("T", " ").replace("Z", " UTC");
}

export default function App() {
  const [settings, setSettings] = useState(() => loadSettings());
  const [health, setHealth] = useState({ state: "idle", message: "" });

  const [createForm, setCreateForm] = useState({
    url: "",
    notes: "",
    expiresAt: ""
  });
  const [createResult, setCreateResult] = useState(null);
  const [createError, setCreateError] = useState("");
  const [createLoading, setCreateLoading] = useState(false);

  const today = useMemo(() => new Date(), []);
  const [statsForm, setStatsForm] = useState({
    slug: "",
    from: toDateInput(new Date(today.getTime() - 6 * MS_PER_DAY)),
    to: toDateInput(today)
  });
  const [statsResult, setStatsResult] = useState(null);
  const [statsError, setStatsError] = useState("");
  const [statsLoading, setStatsLoading] = useState(false);

  const baseUrl = sanitizeBaseUrl(settings.baseUrl) || "http://localhost:8080";

  useEffect(() => {
    saveSettings(settings);
  }, [settings]);

  const maxCount = useMemo(() => {
    if (!statsResult?.daily?.length) return 0;
    return Math.max(...statsResult.daily.map((item) => item.count));
  }, [statsResult]);

  async function handleHealthCheck() {
    setHealth({ state: "loading", message: "Testando..." });
    try {
      const response = await checkHealth(settings);
      const status = response?.data?.status || response?.status || "ok";
      setHealth({ state: "ok", message: `API respondeu: ${status}` });
    } catch (error) {
      setHealth({ state: "error", message: error.message || "Falha ao conectar" });
    }
  }

  async function handleCreate(event) {
    event.preventDefault();
    setCreateError("");
    setCreateResult(null);

    if (!createForm.url.trim()) {
      setCreateError("Informe uma URL válida.");
      return;
    }

    const payload = {
      url: createForm.url.trim()
    };

    if (createForm.notes.trim()) {
      payload.notes = createForm.notes.trim();
    }

    const expiresAtIso = toISODateTime(createForm.expiresAt);
    if (expiresAtIso) {
      payload.expiresAt = expiresAtIso;
    }

    setCreateLoading(true);
    try {
      const data = await createLink(settings, payload);
      setCreateResult(data);
    } catch (error) {
      setCreateError(error.message || "Falha ao criar link.");
    } finally {
      setCreateLoading(false);
    }
  }

  async function handleStats(event) {
    event.preventDefault();
    setStatsError("");
    setStatsResult(null);

    if (!statsForm.slug.trim()) {
      setStatsError("Informe o slug.");
      return;
    }

    if (!statsForm.from || !statsForm.to) {
      setStatsError("Informe o intervalo de datas.");
      return;
    }

    setStatsLoading(true);
    try {
      const data = await fetchStats(
        settings,
        statsForm.slug.trim(),
        statsForm.from,
        statsForm.to
      );
      setStatsResult(data);
    } catch (error) {
      setStatsError(error.message || "Falha ao buscar estatísticas.");
    } finally {
      setStatsLoading(false);
    }
  }

  async function handleCopy(value) {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      // Clipboard may be blocked; ignore.
    }
  }

  return (
    <div className="page">
      <header className="hero">
        <div className="hero-left reveal" style={{ "--delay": "40ms" }}>
          <p className="eyebrow">Encurtador local</p>
          <h1>Encurtador de URLs</h1>
          <p className="subtitle">
            Cliente desktop para criar links curtos e acompanhar cliques.
          </p>
          <div className="hero-badges">
            <span className="badge">Desktop</span>
            <span className="badge">Local API</span>
            <span className="badge">Kong-ready</span>
          </div>
        </div>
        <div className="hero-right reveal" style={{ "--delay": "120ms" }}>
          <div className="status-panel">
            <div className="status-label">Base URL</div>
            <div className="status-value mono">{baseUrl}</div>
            <button type="button" className="ghost" onClick={handleHealthCheck}>
              Testar API
            </button>
            {health.state !== "idle" && (
              <span className={`chip ${health.state}`}>{health.message}</span>
            )}
          </div>
          <div className="hero-note">
            <div className="note-title">Fluxo rápido</div>
            <p>
              <strong>1.</strong> Configure o endpoint.
            </p>
            <p>
              <strong>2.</strong> Gere o link curto.
            </p>
            <p>
              <strong>3.</strong> Consulte cliques por período.
            </p>
          </div>
        </div>
      </header>

      <div className="layout">
        <div className="stack">
          <section className="card" style={{ "--delay": "160ms" }}>
            <h2>Configuração</h2>
            <p className="muted">
              Ajuste o endpoint do REST e headers opcionais (ex.: Kong exige{" "}
              <code>X-User</code>).
            </p>
            <div className="field">
              <label>Base URL</label>
              <input
                type="text"
                value={settings.baseUrl}
                onChange={(event) =>
                  setSettings((prev) => ({
                    ...prev,
                    baseUrl: event.target.value
                  }))
                }
                placeholder="http://localhost:8080"
              />
            </div>
            <div className="field">
              <label>X-API-Key</label>
              <input
                type="password"
                value={settings.apiKey}
                onChange={(event) =>
                  setSettings((prev) => ({
                    ...prev,
                    apiKey: event.target.value
                  }))
                }
                placeholder="opcional"
              />
            </div>
            <div className="field">
              <label>X-User</label>
              <input
                type="text"
                value={settings.user}
                onChange={(event) =>
                  setSettings((prev) => ({
                    ...prev,
                    user: event.target.value
                  }))
                }
                placeholder="demo"
              />
            </div>
            <p className="hint">Preferencias salvas localmente no app.</p>
          </section>

          <section className="card" style={{ "--delay": "240ms" }}>
            <h2>Criar link curto</h2>
            <form onSubmit={handleCreate}>
              <div className="field">
                <label>URL destino</label>
                <input
                  type="url"
                  required
                  value={createForm.url}
                  onChange={(event) =>
                    setCreateForm((prev) => ({
                      ...prev,
                      url: event.target.value
                    }))
                  }
                  placeholder="https://example.com"
                />
              </div>
              <div className="field">
                <label>Notas</label>
                <textarea
                  rows={2}
                  value={createForm.notes}
                  onChange={(event) =>
                    setCreateForm((prev) => ({
                      ...prev,
                      notes: event.target.value
                    }))
                  }
                  placeholder="campanha bairro centro"
                />
              </div>
              <div className="field">
                <label>Expira em</label>
                <input
                  type="datetime-local"
                  value={createForm.expiresAt}
                  onChange={(event) =>
                    setCreateForm((prev) => ({
                      ...prev,
                      expiresAt: event.target.value
                    }))
                  }
                />
              </div>
              <div className="actions">
                <button type="submit" disabled={createLoading}>
                  {createLoading ? "Criando..." : "Criar link"}
                </button>
                {createError && <span className="error">{createError}</span>}
              </div>
            </form>

            {createResult && (
              <div className="result">
                <div>
                  <div className="result-label">Slug</div>
                  <div className="result-value">{createResult.slug}</div>
                </div>
                <div>
                  <div className="result-label">Short URL</div>
                  <div className="result-row">
                    <span className="result-value mono">
                      {createResult.shortUrl}
                    </span>
                    <button
                      type="button"
                      className="ghost"
                      onClick={() => handleCopy(createResult.shortUrl)}
                    >
                      Copiar
                    </button>
                  </div>
                </div>
                <div className="result-grid">
                  <div>
                    <div className="result-label">Criado</div>
                    <div className="result-value">
                      {formatMaybe(createResult.createdAt)}
                    </div>
                  </div>
                  <div>
                    <div className="result-label">Expira</div>
                    <div className="result-value">
                      {formatMaybe(createResult.expiresAt)}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </section>
        </div>

        <div className="stack">
          <section className="card" style={{ "--delay": "320ms" }}>
            <h2>Estatísticas</h2>
            <form onSubmit={handleStats}>
              <div className="field">
                <label>Slug</label>
                <input
                  type="text"
                  value={statsForm.slug}
                  onChange={(event) =>
                    setStatsForm((prev) => ({
                      ...prev,
                      slug: event.target.value
                    }))
                  }
                  placeholder="aZ81kP"
                />
              </div>
              <div className="field-row">
                <div className="field">
                  <label>De</label>
                  <input
                    type="date"
                    value={statsForm.from}
                    onChange={(event) =>
                      setStatsForm((prev) => ({
                        ...prev,
                        from: event.target.value
                      }))
                    }
                  />
                </div>
                <div className="field">
                  <label>Até</label>
                  <input
                    type="date"
                    value={statsForm.to}
                    onChange={(event) =>
                      setStatsForm((prev) => ({
                        ...prev,
                        to: event.target.value
                      }))
                    }
                  />
                </div>
              </div>
              <div className="actions">
                <button type="submit" disabled={statsLoading}>
                  {statsLoading ? "Buscando..." : "Buscar"}
                </button>
                {statsError && <span className="error">{statsError}</span>}
              </div>
            </form>

            {statsResult && (
              <div className="stats">
                <div className="stats-header">
                  <div>
                    <div className="result-label">Slug</div>
                    <div className="result-value">{statsResult.slug}</div>
                  </div>
                  <div>
                    <div className="result-label">Total</div>
                    <div className="result-value">
                      {statsResult.daily?.reduce(
                        (sum, item) => sum + item.count,
                        0
                      )}
                    </div>
                  </div>
                </div>

                <div className="table">
                  {statsResult.daily?.map((item) => (
                    <div className="table-row" key={item.date}>
                      <span className="mono">{item.date}</span>
                      <div className="bar-track">
                        <div
                          className="bar"
                          style={{
                            width: maxCount
                              ? `${(item.count / maxCount) * 100}%`
                              : "0%"
                          }}
                        />
                      </div>
                      <span className="mono">{item.count}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </section>

          <section className="card subtle" style={{ "--delay": "400ms" }}>
            <h2>Referências rápidas</h2>
            <div className="quick">
              <div>
                <div className="result-label">POST</div>
                <div className="result-value mono">/api/links</div>
              </div>
              <div>
                <div className="result-label">GET</div>
                <div className="result-value mono">/api/links/&#123;slug&#125;/stats</div>
              </div>
              <div>
                <div className="result-label">GET</div>
                <div className="result-value mono">/&#123;slug&#125;</div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
