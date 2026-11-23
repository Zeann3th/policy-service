// Suite 05 soak: a steady rate below the target for a long time, to expose
// leaks (memory, goroutines, connections) and slow drift (bloat, partition
// growth). Run scripts/monitor.sh alongside and compare the first and last
// samples.
//
//   k6 run e2e/k6/soak.js
//   k6 run -e RATE=600 -e DURATION=1h e2e/k6/soak.js
import { pair, release, reserve, reserveBody, summary } from './lib.js';

const RATE = Number(__ENV.RATE || 600);
const DURATION = __ENV.DURATION || '30m';
const RELEASE_RATIO = Number(__ENV.RELEASE_RATIO || 0.1);

export const options = {
  scenarios: {
    soak: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 200,
      maxVUs: 1500,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.001'],
    unexpected_responses: ['rate<0.001'],
    reserve_latency: ['p(95)<200', 'p(99)<500'],
  },
};

const reservedIds = []; // per VU

export default function () {
  if (reservedIds.length > 0 && Math.random() < RELEASE_RATIO) {
    release(reservedIds.shift());
    return;
  }
  const [sender, receiver] = pair();
  const response = reserve(reserveBody({ sender, receiver }));
  if (response.status === 200 && response.json('status') === 'RESERVED') {
    reservedIds.push(response.json('requestId'));
  }
}

export const handleSummary = summary('soak');
