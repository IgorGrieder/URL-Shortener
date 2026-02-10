import http from "k6/http";
import { check, fail } from "k6";
import { Counter } from "k6/metrics";

const baseURL = (__ENV.LT_BASE_URL || "http://localhost:8080").trim();
const mode = (__ENV.LT_MODE || "mixed").trim().toLowerCase();
const xUser = (__ENV.LT_X_USER || "k6-load").trim();
const apiKey = (__ENV.LT_API_KEY || "").trim();
const createURL = (__ENV.LT_CREATE_URL || "https://example.com").trim();
const allowExtremeLocalhost =
  (__ENV.LT_ALLOW_EXTREME_LOCALHOST || "false").trim().toLowerCase() === "true";

const targetTPSRaw = Number(__ENV.LT_TARGET_TPS || "1000");
const targetTPS = Math.floor(targetTPSRaw);
const duration = (__ENV.LT_DURATION || "1m").trim();
const preAllocatedVUs = Math.floor(
  Number(__ENV.LT_PRE_ALLOCATED_VUS || "400"),
);
const maxVUs = Math.floor(Number(__ENV.LT_MAX_VUS || "4000"));
const httpTimeout = (__ENV.LT_HTTP_TIMEOUT || "5s").trim();
const seedLinks = Number(__ENV.LT_SEED_LINKS || "20");

const mixedCreatePct = Number(__ENV.LT_MIXED_CREATE_PCT || "10");
const mixedRedirectPct = Number(__ENV.LT_MIXED_REDIRECT_PCT || "80");
const mixedStatsPct = Number(__ENV.LT_MIXED_STATS_PCT || "10");
const mixedHealthPct = Number(__ENV.LT_MIXED_HEALTH_PCT || "0");

function parseStatusCSV(rawValue, fallbackValue) {
  return (rawValue || fallbackValue)
    .split(",")
    .map((value) => Number(value.trim()))
    .filter((value) => Number.isInteger(value) && value > 0);
}

const expectedCreateStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_CREATE_STATUSES,
  "201",
);
const expectedRedirectStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_REDIRECT_STATUSES,
  "301,302",
);
const expectedStatsStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_STATS_STATUSES,
  "200",
);
const expectedHealthStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_HEALTH_STATUSES,
  "200",
);

const endpointCreate = new Counter("endpoint_create_requests");
const endpointRedirect = new Counter("endpoint_redirect_requests");
const endpointStats = new Counter("endpoint_stats_requests");
const endpointHealth = new Counter("endpoint_health_requests");

const status2xx = new Counter("status_2xx");
const status3xx = new Counter("status_3xx");
const status4xx = new Counter("status_4xx");
const status5xx = new Counter("status_5xx");

const validModes = ["mixed", "create", "redirect", "stats", "health"];
if (!validModes.includes(mode)) {
  fail(`LT_MODE must be one of: ${validModes.join(", ")}`);
}
if (!Number.isFinite(targetTPSRaw) || targetTPS < 1) {
  fail("LT_TARGET_TPS must be a positive number (>= 1)");
}
if (!Number.isFinite(preAllocatedVUs) || preAllocatedVUs <= 0) {
  fail("LT_PRE_ALLOCATED_VUS must be a positive number");
}
if (!Number.isFinite(maxVUs) || maxVUs <= 0) {
  fail("LT_MAX_VUS must be a positive number");
}
if (!Number.isFinite(seedLinks) || seedLinks < 0) {
  fail("LT_SEED_LINKS must be zero or a positive number");
}
const localhostPattern = /^https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?(?:\/|$)/i;
if (localhostPattern.test(baseURL) && (targetTPS > 10000 || maxVUs > 12000)) {
  const msg =
    "High LT_TARGET_TPS/LT_MAX_VUS against localhost may exhaust client ephemeral ports and cause 'connect: can't assign requested address'. Lower LT_TARGET_TPS/LT_MAX_VUS or run distributed load generators.";
  if (allowExtremeLocalhost) {
    console.warn(msg);
  } else {
    fail(`${msg} If you really want this, set LT_ALLOW_EXTREME_LOCALHOST=true.`);
  }
}
if (expectedCreateStatuses.length === 0) {
  fail("LT_EXPECTED_CREATE_STATUSES must contain at least one status code");
}
if (expectedRedirectStatuses.length === 0) {
  fail("LT_EXPECTED_REDIRECT_STATUSES must contain at least one status code");
}
if (expectedStatsStatuses.length === 0) {
  fail("LT_EXPECTED_STATS_STATUSES must contain at least one status code");
}
if (expectedHealthStatuses.length === 0) {
  fail("LT_EXPECTED_HEALTH_STATUSES must contain at least one status code");
}

function buildHeaders(extra = {}) {
  const headers = {
    "X-User": xUser,
    ...extra,
  };
  if (apiKey !== "") {
    headers["X-API-Key"] = apiKey;
  }
  return headers;
}

function normalizePath(path) {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  if (path.startsWith("/")) {
    return path;
  }
  return `/${path}`;
}

function buildURL(path) {
  if (path.startsWith("http://") || path.startsWith("https://")) {
    return path;
  }
  return `${baseURL}${normalizePath(path)}`;
}

function requestParams(tags, extraHeaders = {}, keepResponseBody = false) {
  const params = {
    headers: buildHeaders(extraHeaders),
    redirects: 0,
    timeout: httpTimeout,
    tags: {
      load_test: "api_gateway_tps",
      mode: mode,
      ...tags,
    },
  };
  if (keepResponseBody) {
    params.responseType = "text";
  }
  return params;
}

function recordStatus(status) {
  if (status >= 200 && status < 300) {
    status2xx.add(1);
    return;
  }
  if (status >= 300 && status < 400) {
    status3xx.add(1);
    return;
  }
  if (status >= 400 && status < 500) {
    status4xx.add(1);
    return;
  }
  if (status >= 500) {
    status5xx.add(1);
  }
}

function randomSlug(setupData) {
  const slugs = setupData.seededSlugs || [];
  if (slugs.length === 0) {
    fail("no seeded slugs available, increase LT_SEED_LINKS or choose LT_MODE=create/health");
  }
  return slugs[Math.floor(Math.random() * slugs.length)];
}

function createLinkURL(suffix) {
  return `${createURL}?k6=${suffix}`;
}

function createOneLink(urlValue, keepResponseBody = false) {
  const path = "/api/links";
  const payload = JSON.stringify({ url: urlValue });
  return http.post(
    buildURL(path),
    payload,
    requestParams(
      {
        endpoint: "create",
        path: path,
      },
      { "Content-Type": "application/json" },
      keepResponseBody,
    ),
  );
}

function responseJSON(response, endpointName) {
  try {
    return response.json();
  } catch (error) {
    fail(`${endpointName} response is not valid JSON (${String(error)})`);
  }
}

function extractSlugFromCreate(response) {
  const body = responseJSON(response, "create");
  const slug = body && body.data && body.data.slug;
  if (typeof slug !== "string" || slug.trim() === "") {
    fail("create response missing data.slug");
  }
  return slug;
}

function ensureSeedLinks() {
  const slugs = [];
  for (let i = 0; i < seedLinks; i += 1) {
    const response = createOneLink(createLinkURL(`seed-${Date.now()}-${i}`), true);

    if (response.status === 401) {
      fail("setup failed: POST /api/links returned 401, set LT_API_KEY if API keys are enabled");
    }
    if (response.status === 429) {
      fail("setup failed: POST /api/links returned 429, increase CREATE_RATE_LIMIT_PER_MINUTE for insertion tests");
    }
    if (response.status !== 201) {
      fail(`setup failed: POST /api/links returned ${response.status}, expected 201`);
    }

    slugs.push(extractSlugFromCreate(response));
  }
  return slugs;
}

function needsSeedLinks() {
  return mode === "mixed" || mode === "redirect" || mode === "stats";
}

function todayDate() {
  return new Date().toISOString().slice(0, 10);
}

function smokeCreate() {
  const response = createOneLink(createLinkURL(`smoke-${Date.now()}`), true);
  if (!expectedCreateStatuses.includes(response.status)) {
    fail(
      `setup failed: ${buildURL("/api/links")} returned ${response.status}, expected one of [${expectedCreateStatuses.join(", ")}]`,
    );
  }
}

function smokeRedirect(slug) {
  const path = `/${slug}`;
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "redirect",
      path: path,
    }),
  );
  if (!expectedRedirectStatuses.includes(response.status)) {
    fail(
      `setup failed: ${buildURL(path)} returned ${response.status}, expected one of [${expectedRedirectStatuses.join(", ")}]`,
    );
  }
}

function smokeStats(slug, fromDate, toDate) {
  const path = `/api/links/${slug}/stats?from=${fromDate}&to=${toDate}`;
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "stats",
      path: "/api/links/{slug}/stats",
    }),
  );
  if (!expectedStatsStatuses.includes(response.status)) {
    fail(
      `setup failed: ${buildURL(path)} returned ${response.status}, expected one of [${expectedStatsStatuses.join(", ")}]`,
    );
  }
}

function smokeHealth() {
  const path = "/health";
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "health",
      path: path,
    }),
  );
  if (!expectedHealthStatuses.includes(response.status)) {
    fail(
      `setup failed: ${buildURL(path)} returned ${response.status}, expected one of [${expectedHealthStatuses.join(", ")}]`,
    );
  }
}

function buildScenario(rate, execName) {
  const ratio = rate / targetTPS;
  const scenarioPreAllocated = Math.max(
    1,
    Math.ceil(preAllocatedVUs * ratio),
  );
  const scenarioMax = Math.max(
    scenarioPreAllocated,
    Math.ceil(maxVUs * ratio),
  );

  return {
    executor: "constant-arrival-rate",
    rate: rate,
    timeUnit: "1s",
    duration: duration,
    preAllocatedVUs: scenarioPreAllocated,
    maxVUs: scenarioMax,
    exec: execName,
  };
}

function allocateMixedRates() {
  const parts = [
    { name: "create", weight: mixedCreatePct },
    { name: "redirect", weight: mixedRedirectPct },
    { name: "stats", weight: mixedStatsPct },
    { name: "health", weight: mixedHealthPct },
  ].filter((entry) => Number.isFinite(entry.weight) && entry.weight > 0);

  if (parts.length === 0) {
    fail("mixed mode requires at least one positive LT_MIXED_*_PCT");
  }

  const rates = {};
  let remainingRate = Math.floor(targetTPS);
  let remainingWeight = parts.reduce((sum, part) => sum + part.weight, 0);

  parts.forEach((part, idx) => {
    let rate = 0;
    const isLast = idx === parts.length - 1;

    if (isLast) {
      rate = remainingRate;
    } else {
      rate = Math.floor((remainingRate * part.weight) / remainingWeight);
    }

    if (rate === 0 && remainingRate > 0) {
      rate = 1;
    }
    if (rate > remainingRate) {
      rate = remainingRate;
    }

    rates[part.name] = rate;
    remainingRate -= rate;
    remainingWeight -= part.weight;
  });

  return rates;
}

function buildScenarios() {
  if (mode === "create") {
    return { create: buildScenario(targetTPS, "createScenario") };
  }
  if (mode === "redirect") {
    return { redirect: buildScenario(targetTPS, "redirectScenario") };
  }
  if (mode === "stats") {
    return { stats: buildScenario(targetTPS, "statsScenario") };
  }
  if (mode === "health") {
    return { health: buildScenario(targetTPS, "healthScenario") };
  }

  const mixedRates = allocateMixedRates();
  const scenarios = {};

  if (mixedRates.create > 0) {
    scenarios.create = buildScenario(mixedRates.create, "createScenario");
  }
  if (mixedRates.redirect > 0) {
    scenarios.redirect = buildScenario(mixedRates.redirect, "redirectScenario");
  }
  if (mixedRates.stats > 0) {
    scenarios.stats = buildScenario(mixedRates.stats, "statsScenario");
  }
  if (mixedRates.health > 0) {
    scenarios.health = buildScenario(mixedRates.health, "healthScenario");
  }

  if (Object.keys(scenarios).length === 0) {
    fail("no mixed scenario has positive rate after allocation");
  }

  return scenarios;
}

export const options = {
  scenarios: buildScenarios(),
  discardResponseBodies: true,
  thresholds: {
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};

export function setup() {
  const fromDate = todayDate();
  const toDate = fromDate;
  const setupData = {
    seededSlugs: [],
    fromDate: fromDate,
    toDate: toDate,
  };

  if (needsSeedLinks()) {
    setupData.seededSlugs = ensureSeedLinks();
  }

  if (mode === "create") {
    smokeCreate();
    return setupData;
  }
  if (mode === "redirect") {
    smokeRedirect(randomSlug(setupData));
    return setupData;
  }
  if (mode === "stats") {
    const slug = randomSlug(setupData);
    smokeStats(slug, fromDate, toDate);
    return setupData;
  }
  if (mode === "health") {
    smokeHealth();
    return setupData;
  }

  smokeCreate();
  const slug = randomSlug(setupData);
  smokeRedirect(slug);
  smokeStats(slug, fromDate, toDate);
  if (mixedHealthPct > 0) {
    smokeHealth();
  }

  return setupData;
}

export function createScenario() {
  endpointCreate.add(1);

  const unique = `${Date.now()}-${Math.random()}-${__VU}-${__ITER}`;
  const response = createOneLink(createLinkURL(`run-${unique}`));
  recordStatus(response.status);

  check(response, {
    "create status in expected set": (r) => expectedCreateStatuses.includes(r.status),
  });
}

export function redirectScenario(setupData) {
  endpointRedirect.add(1);

  const slug = randomSlug(setupData);
  const path = `/${slug}`;
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "redirect",
      path: path,
    }),
  );
  recordStatus(response.status);

  check(response, {
    "redirect status in expected set": (r) =>
      expectedRedirectStatuses.includes(r.status),
  });
}

export function statsScenario(setupData) {
  endpointStats.add(1);

  const slug = randomSlug(setupData);
  const path = `/api/links/${slug}/stats?from=${setupData.fromDate}&to=${setupData.toDate}`;
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "stats",
      path: "/api/links/{slug}/stats",
    }),
  );
  recordStatus(response.status);

  check(response, {
    "stats status in expected set": (r) => expectedStatsStatuses.includes(r.status),
  });
}

export function healthScenario() {
  endpointHealth.add(1);

  const path = "/health";
  const response = http.get(
    buildURL(path),
    requestParams({
      endpoint: "health",
      path: path,
    }),
  );
  recordStatus(response.status);

  check(response, {
    "health status in expected set": (r) => expectedHealthStatuses.includes(r.status),
  });
}
