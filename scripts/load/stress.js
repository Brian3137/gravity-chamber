import http from 'k6/http';
import { Trend } from 'k6/metrics';

// OPEN model: requests arrive at a fixed rate whether or not the server keeps
// up. This is what real traffic does, and it is the only way to see the cliff.
export const options = {
  scenarios: {
    ramp: {
      executor: 'ramping-arrival-rate',
      startRate: 20,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 800,          // must be high enough not to become the bottleneck
      stages: [
        { target: 50,   duration: '30s' },
        { target: 100,  duration: '30s' },
        { target: 200,  duration: '30s' },
        { target: 400,  duration: '30s' },
        { target: 800,  duration: '30s' },
        { target: 1600, duration: '30s' },
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