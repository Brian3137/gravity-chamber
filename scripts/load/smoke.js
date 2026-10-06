import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '20s',
  thresholds: {
    // Thresholds are pass/fail criteria. A failed threshold exits non-zero,
    // which is what makes this runnable in CI.
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<200'],
  },
};

const BASE = __ENV.BASE || 'http://localhost:8080';

export default function () {
  const res = http.get(`${BASE}/leaderboard`);
  check(res, {
    'status 200': (r) => r.status === 200,
    'has body': (r) => r.body.length > 0,
  });
}