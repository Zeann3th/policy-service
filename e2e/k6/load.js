// Suite 02 load: a constant arrival rate (default 1000 requests/s) for a
// fixed duration over all 1000 wallets. Each iteration is one request: a
// reserve, or (RELEASE_RATIO of the time) the release of a reservation this
// VU made earlier.
//
//   k6 run e2e/k6/load.js
//   k6 run -e RATE=1000 -e DURATION=5m -e RELEASE_RATIO=0.1 e2e/k6/load.js
import { pair, release, reserve, reserveBody, summary } from './lib.js';

const RATE = Number(__ENV.RATE || 1000);
const DURATION = __ENV.DURATION || '5m';
const RELEASE_RATIO = Number(__ENV.RELEASE_RATIO || 0.1);

export const options = {
  scenarios: {
    load: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Number(__ENV.VUS || 200),
      maxVUs: Number(__ENV.MAX_VUS || 2000),
    },
  },
  thresholds: {
    // sustained the target: under 1% of iterations dropped for lack of VUs
    dropped_iterations: [`count<${Math.ceil(RATE * 0.01 * 60)}`],
    http_req_failed: ['rate<0.001'],
    unexpected_responses: ['rate<0.001'],
    reserve_latency: ['p(95)<200', 'p(99)<500'],
    release_latency: ['p(95)<200', 'p(99)<500'],
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

export const handleSummary = summary('load');
