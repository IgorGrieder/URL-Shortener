import http from "k6/http";
import { check, fail } from "k6";
import { Counter } from "k6/metrics";

const baseURL = (__ENV.LT_BASE_URL || "http://localhost:8080").trim();
const xUser = (__ENV.LT_X_USER || "k6-crud").trim();
const apiKey = (__ENV.LT_API_KEY || "").trim();
const httpTimeout = (__ENV.LT_HTTP_TIMEOUT || "10s").trim();

const vusRaw = Number(__ENV.LT_VUS || "5");
const vus = Math.floor(vusRaw);
const iterationsRaw = Number(__ENV.LT_ITERATIONS || "30");
const iterations = Math.floor(iterationsRaw);
const maxDuration = (__ENV.LT_MAX_DURATION || "2m").trim();

function parseStatusCSV(rawValue, fallbackValue) {
  return (rawValue || fallbackValue)
    .split(",")
    .map((value) => Number(value.trim()))
    .filter((value) => Number.isInteger(value) && value > 0);
}

const expectedRedirectStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_REDIRECT_STATUSES,
  "301,302",
);
const expectedDeletedStatuses = parseStatusCSV(
  __ENV.LT_EXPECTED_DELETED_STATUSES,
  "404",
);
const expectedCreateStatus = http.expectedStatuses(201);
const expectedStatsStatus = http.expectedStatuses(200);
const expectedDeleteStatus = http.expectedStatuses(200);
const expectedRedirectStatusesCallback = http.expectedStatuses(
  ...expectedRedirectStatuses,
);
const expectedDeletedStatusesCallback = http.expectedStatuses(
  ...expectedDeletedStatuses,
);

const endpointCreate = new Counter("endpoint_create_requests");
const endpointReadRedirect = new Counter("endpoint_read_redirect_requests");
const endpointReadStats = new Counter("endpoint_read_stats_requests");
const endpointDelete = new Counter("endpoint_delete_requests");
const endpointReadAfterDelete = new Counter("endpoint_read_after_delete_requests");

if (!Number.isFinite(vusRaw) || vus < 1) {
  fail("LT_VUS must be a positive number");
}
if (!Number.isFinite(iterationsRaw) || iterations < 1) {
  fail("LT_ITERATIONS must be a positive number");
}
if (expectedRedirectStatuses.length === 0) {
  fail("LT_EXPECTED_REDIRECT_STATUSES must contain at least one status code");
}
if (expectedDeletedStatuses.length === 0) {
  fail("LT_EXPECTED_DELETED_STATUSES must contain at least one status code");
}

export const options = {
  scenarios: {
    crud: {
      executor: "shared-iterations",
      vus: vus,
      iterations: iterations,
      maxDuration: maxDuration,
      exec: "crudScenario",
    },
  },
  discardResponseBodies: false,
  thresholds: {
    http_req_failed: ["rate<0.01"],
    checks: ["rate>0.99"],
  },
};

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

function requestParams(
  tags,
  extraHeaders = {},
  keepResponseBody = false,
  responseCallback = null,
) {
  const params = {
    headers: buildHeaders(extraHeaders),
    redirects: 0,
    timeout: httpTimeout,
    tags: {
      load_test: "api_gateway_crud",
      ...tags,
    },
  };
  if (responseCallback) {
    params.responseCallback = responseCallback;
  }
  if (keepResponseBody) {
    params.responseType = "text";
  }
  return params;
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

function todayDate() {
  return new Date().toISOString().slice(0, 10);
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

export function setup() {
  const response = http.get(
    buildURL("/health"),
    requestParams({ endpoint: "health", path: "/health" }),
  );

  if (response.status === 401) {
    fail("setup failed: gateway denied request (check LT_X_USER header)");
  }
  if (response.status !== 200) {
    fail(`setup failed: /health returned ${response.status}, expected 200`);
  }

  return { today: todayDate() };
}

export function crudScenario(setupData) {
  const unique = `${Date.now()}-${__VU}-${__ITER}-${Math.random()}`;
  const payload = JSON.stringify({ url: `https://example.com/crud/${unique}` });

  const createPath = "/api/links";
  const createResponse = http.post(
    buildURL(createPath),
    payload,
    requestParams(
      {
        endpoint: "create",
        path: createPath,
      },
      { "Content-Type": "application/json" },
      true,
      expectedCreateStatus,
    ),
  );
  endpointCreate.add(1);

  check(createResponse, {
    "create status is 201": (r) => r.status === 201,
  });

  if (createResponse.status !== 201) {
    return;
  }

  const slug = extractSlugFromCreate(createResponse);

  const redirectPath = `/${slug}`;
  const redirectResponse = http.get(
    buildURL(redirectPath),
    requestParams(
      { endpoint: "redirect_read", path: redirectPath },
      {},
      false,
      expectedRedirectStatusesCallback,
    ),
  );
  endpointReadRedirect.add(1);

  check(redirectResponse, {
    "redirect read status in expected set": (r) =>
      expectedRedirectStatuses.includes(r.status),
  });

  const statsPath = `/api/links/${slug}/stats?from=${setupData.today}&to=${setupData.today}`;
  const statsResponse = http.get(
    buildURL(statsPath),
    requestParams(
      { endpoint: "stats_read", path: "/api/links/{slug}/stats" },
      {},
      false,
      expectedStatsStatus,
    ),
  );
  endpointReadStats.add(1);

  check(statsResponse, {
    "stats read status is 200": (r) => r.status === 200,
  });

  const deletePath = `/api/links/${slug}`;
  const deleteResponse = http.del(
    buildURL(deletePath),
    null,
    requestParams(
      { endpoint: "delete", path: "/api/links/{slug}" },
      {},
      true,
      expectedDeleteStatus,
    ),
  );
  endpointDelete.add(1);

  check(deleteResponse, {
    "delete status is 200": (r) => r.status === 200,
  });

  const deletedRedirectResponse = http.get(
    buildURL(redirectPath),
    requestParams(
      { endpoint: "redirect_read_after_delete", path: redirectPath },
      {},
      false,
      expectedDeletedStatusesCallback,
    ),
  );
  endpointReadAfterDelete.add(1);

  check(deletedRedirectResponse, {
    "redirect after delete status in expected set": (r) =>
      expectedDeletedStatuses.includes(r.status),
  });

  const deletedStatsResponse = http.get(
    buildURL(statsPath),
    requestParams(
      { endpoint: "stats_read_after_delete", path: "/api/links/{slug}/stats" },
      {},
      false,
      expectedDeletedStatusesCallback,
    ),
  );
  endpointReadAfterDelete.add(1);

  check(deletedStatsResponse, {
    "stats after delete status in expected set": (r) =>
      expectedDeletedStatuses.includes(r.status),
  });
}
