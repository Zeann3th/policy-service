// Suite 04 contention: the worst case for the wallet locks. All traffic hits
// HOT wallets (default 10) in both directions, so every request competes
// for the same advisory locks in opposite orders, while a second scenario
// releases each reservation from several VUs at once (all must get the same
// 200 response: release is idempotent). A deadlock surfaces as
// a 500 (SQLSTATE 40P01) and in pg_stat_database.deadlocks (verify.sh).
//
//   k6 run e2e/k6/contention.js
//   k6 run -e HOT=4 -e RATE=400 e2e/k6/contention.js
import { check } from 'k6';
import http from 'k6/http';
import { RESERVATIONS, requestId, reserve, reserveBody, summary, wallet } from './lib.js';

const HOT = Number(__ENV.HOT || 10);
const RATE = Number(__ENV.RATE || 300);
const DURATION = __ENV.DURATION || '2m';
const RACERS = 4; // concurrent releases of the same request

// T3 wallets without custom policies (603, 606, ...): 40,000 transactions a
// day each, so the hot set stays mostly RESERVED. Rejections would still be
// fine for the locks: a rejected request takes both wallet locks too.
const HOT_WALLETS = Array.from({ length: HOT }, (_, i) => wallet(603 + 3 * i));

function hotPair() {
  const sender = Math.floor(Math.random() * HOT);
  let receiver = Math.floor(Math.random() * HOT);
  while (receiver === sender) {
    receiver = Math.floor(Math.random() * HOT);
  }
  return [HOT_WALLETS[sender], HOT_WALLETS[receiver]];
}

export const options = {
  scenarios: {
    opposing: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 100,
      maxVUs: 1000,
      exec: 'opposing',
    },
    racingRelease: {
      executor: 'constant-arrival-rate',
      rate: Math.max(1, Math.floor(RATE / 10)),
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: 50,
      maxVUs: 500,
      exec: 'racingRelease',
    },
  },
  thresholds: {
    'http_req_failed{name:reserve}': ['rate==0'],
    unexpected_responses: ['rate==0'],
    'checks{check:racing releases answer alike}': ['rate==1'],
    reserve_latency: ['p(99)<2000'],
  },
};

export function opposing() {
  const [sender, receiver] = hotPair();
  reserve(reserveBody({ sender, receiver, amount: 1000 }), { scenario: 'opposing' });
}

export function racingRelease() {
  const [sender, receiver] = hotPair();
  const id = requestId('race');
  const response = reserve(reserveBody({ sender, receiver, amount: 1000, id }), { scenario: 'racing' });
  if (response.status !== 200 || response.json('status') !== 'RESERVED') {
    return;
  }
  const requests = [];
  for (let i = 0; i < RACERS; i++) {
    requests.push(['DELETE', `${RESERVATIONS}/${encodeURIComponent(id)}`, null, { tags: { name: 'release' } }]);
  }
  const responses = http.batch(requests);
  check(responses, {
    'racing releases answer alike': (rs) => rs.every((r) => r.status === 200 && r.body === rs[0].body),
  });
}

export const handleSummary = summary('contention');
