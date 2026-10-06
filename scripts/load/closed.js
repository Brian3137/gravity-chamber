import http from 'k6/http';
import { Trend } from 'k6/metrics';

// CLOSED model: users wait for each response before continuing.
export const options = {
  scenarios: {
        ramp: {
      executor: 'ramping-vus',      
      startVUs: 10,
      stages: [
        { target: 50,  duration: '30s' },
        { target: 200, duration: '30s' },
        { target: 800, duration: '30s' },
      ],
    },
  },
};

const BASE = __ENV.BASE || 'http://localhost:8080';
const write = new Trend('write_latency', true);

export default function () {
  // A read-heavy mix with some writes — closer to real traffic than either alone.
  if (Math.random() < 0.1) {
    const r = http.post(`${BASE}/readings`,
      JSON.stringify({ fighter: `f${Math.floor(Math.random() * 1000)}`,
                       power: Math.floor(Math.random() * 100000) }),
      { headers: { 'Content-Type': 'application/json' } });
    write.add(r.timings.duration);
  } else {
    http.get(`${BASE}/leaderboard?limit=10`);
  }
}