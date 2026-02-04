const STORAGE_KEY = "encurtador-url.settings.v1";

const ENV_DEFAULTS = {
  baseUrl: import.meta.env.VITE_API_BASE_URL || "http://localhost:8080",
  apiKey: import.meta.env.VITE_API_KEY || "",
  user: import.meta.env.VITE_API_USER || ""
};

export const DEFAULT_SETTINGS = {
  baseUrl: ENV_DEFAULTS.baseUrl,
  apiKey: ENV_DEFAULTS.apiKey,
  user: ENV_DEFAULTS.user
};

export function sanitizeBaseUrl(value) {
  if (!value) return "";
  return value.trim().replace(/\/+$/, "");
}

export function loadSettings() {
  if (typeof localStorage === "undefined") return { ...DEFAULT_SETTINGS };
  try {
    const saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
    return { ...DEFAULT_SETTINGS, ...(saved || {}) };
  } catch {
    return { ...DEFAULT_SETTINGS };
  }
}

export function saveSettings(settings) {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}

function buildHeaders(settings) {
  const headers = {
    "Content-Type": "application/json"
  };

  if (settings.apiKey) {
    headers["X-API-Key"] = settings.apiKey;
  }

  if (settings.user) {
    headers["X-User"] = settings.user;
  }

  return headers;
}

async function requestJson(url, options) {
  const response = await fetch(url, options);
  const text = await response.text();
  const payload = text ? safeParse(text) : null;

  if (!response.ok) {
    const message = payload?.message || payload?.error || `HTTP ${response.status}`;
    const error = new Error(message);
    error.payload = payload;
    error.status = response.status;
    throw error;
  }

  return payload;
}

function safeParse(text) {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

function normalizeData(payload) {
  if (!payload) return null;
  if (Object.prototype.hasOwnProperty.call(payload, "data")) {
    return payload.data;
  }
  return payload;
}

export async function checkHealth(settings) {
  const baseUrl = sanitizeBaseUrl(settings.baseUrl) || DEFAULT_SETTINGS.baseUrl;
  return requestJson(`${baseUrl}/health`, { method: "GET" });
}

export async function createLink(settings, data) {
  const baseUrl = sanitizeBaseUrl(settings.baseUrl) || DEFAULT_SETTINGS.baseUrl;
  const payload = await requestJson(`${baseUrl}/api/links`, {
    method: "POST",
    headers: buildHeaders(settings),
    body: JSON.stringify(data)
  });
  return normalizeData(payload);
}

export async function fetchStats(settings, slug, from, to) {
  const baseUrl = sanitizeBaseUrl(settings.baseUrl) || DEFAULT_SETTINGS.baseUrl;
  const params = new URLSearchParams();
  if (from) params.set("from", from);
  if (to) params.set("to", to);

  const url = `${baseUrl}/api/links/${encodeURIComponent(slug)}/stats?${params.toString()}`;
  const payload = await requestJson(url, {
    method: "GET",
    headers: buildHeaders(settings)
  });
  return normalizeData(payload);
}
