// Suite 03 stress: ramps the arrival rate past the target to find where
// latency or errors break (the knee). Thresholds are reported, not used to
// abort, so the whole curve is recorded.
//
//   k6 run e2e/k6/stress.js
//   k6 run -e PEAK=4000 -e STEP=2m e2e/k6/stress.js
import { pair, reserve, reserveBody, summary } from './lib.js';

const PEAK = Number(__ENV.PEAK || 3000);
const STEP = __ENV.STEP || '1m';

export const options = {
  scenarios: {
    stress: {
      executor: 'ramping-arrival-rate',
      startRate: 100,
      timeUnit: '1s',
      preAllocatedVUs: 300,
      maxVUs: Number(__ENV.MAX_VUS || 4000),
      stages: [
        { target: 500, duration: STEP },
        { target: 500, duration: STEP },
        { target: 1000, duration: STEP },
        { target: 1000, duration: STEP },
        { target: 1500, duration: STEP },
        { target: 1500, duration: STEP },
        { target: 2000, duration: STEP },
        { target: 2000, duration: STEP },
        { target: PEAK, duration: STEP },
        { target: PEAK, duration: STEP },
        { target: 0, duration: '30s' },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    unexpected_responses: ['rate<0.01'],
    reserve_latency: ['p(95)<500'],
  },
};

export default function () {
  const [sender, receiver] = pair();
  reserve(reserveBody({ sender, receiver }));
}

export const handleSummary = summary('stress');
