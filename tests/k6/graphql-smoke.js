import http from 'k6/http';
import { check, sleep } from 'k6';

const targetVUs = Number(__ENV.K6_TARGET_VUS || 20);
const rampUp = __ENV.K6_RAMP_UP || '30s';
const hold = __ENV.K6_HOLD || '2m';
const rampDown = __ENV.K6_RAMP_DOWN || '30s';

export const options = {
  scenarios: {
    graphql_health: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: rampUp, target: targetVUs },
        { duration: hold, target: targetVUs },
        { duration: rampDown, target: 0 },
      ],
      gracefulRampDown: '10s',
    },
  },
  thresholds: {
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<800'],
  },
};

const baseUrl = __ENV.GOSHOPX_URL || 'http://graphql:8080';

export default function () {
  const response = http.get(`${baseUrl}/health`);
  check(response, { 'health responds 200': (r) => r.status === 200 });
  sleep(1);
}
