import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 2,
  duration: '30s',
  thresholds: { http_req_failed: ['rate<0.01'], http_req_duration: ['p(95)<800'] },
};

const baseUrl = __ENV.GOSHOPX_URL || 'http://goshopx-graphql.goshopx.svc.cluster.local:8080';

export default function () {
  const response = http.get(`${baseUrl}/health`);
  check(response, { 'health responds 200': (r) => r.status === 200 });
  sleep(1);
}
