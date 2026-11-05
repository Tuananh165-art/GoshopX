import http from 'k6/http';
import { check, sleep } from 'k6';

const profile = __ENV.K6_PROFILE || 'smoke';
const stages = {
  smoke: [{ duration: '1m', target: 1 }],
  load: [{ duration: '1m', target: 5 }, { duration: '3m', target: 5 }, { duration: '1m', target: 10 }, { duration: '3m', target: 10 }, { duration: '1m', target: 20 }, { duration: '3m', target: 20 }],
  stress: [{ duration: '1m', target: 40 }, { duration: '2m', target: 40 }, { duration: '1m', target: 80 }, { duration: '2m', target: 80 }, { duration: '1m', target: 160 }, { duration: '2m', target: 160 }],
  soak: [{ duration: '2m', target: Number(__ENV.K6_SOAK_RPS || 10) }, { duration: '30m', target: Number(__ENV.K6_SOAK_RPS || 10) }],
};

if (!stages[profile]) throw new Error(`Unknown K6_PROFILE: ${profile}`);

export const options = {
  scenarios: {
    graphql_catalog: {
      executor: 'ramping-arrival-rate',
      startRate: profile === 'smoke' ? 1 : 5,
      timeUnit: '1s',
      preAllocatedVUs: 20,
      maxVUs: 200,
      stages: stages[profile],
      gracefulStop: '30s',
    },
  },
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: [{ threshold: 'rate<0.05', abortOnFail: true, delayAbortEval: '1m' }],
    http_req_duration: ['p(95)<1500', 'p(99)<3000'],
    dropped_iterations: ['count==0'],
  },
};

const url = __ENV.GOSHOPX_URL || 'http://localhost:8080';
const headers = { 'Content-Type': 'application/json' };
if (__ENV.GOSHOPX_TOKEN) headers.Authorization = `Bearer ${__ENV.GOSHOPX_TOKEN}`;

export default function () {
  const query = `query LoadCatalog($pagination: PaginationInput, $query: String) {
    product(pagination: $pagination, query: $query) { id name price thumbnail stock }
  }`;
  const response = http.post(url, JSON.stringify({ query, variables: { pagination: { skip: 0, take: 20 }, query: 'phone' } }), { headers, tags: { operation: 'catalog_search' } });
  let body;
  try { body = response.json(); } catch { body = null; }
  check(response, {
    'GraphQL HTTP 200': (r) => r.status === 200,
    'GraphQL has no errors': () => Array.isArray(body?.errors) === false,
    'catalog response shape': () => Array.isArray(body?.data?.product),
  });
  sleep(0.2);
}
