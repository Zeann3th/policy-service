// Shared helpers for the e2e k6 suites. The wallet scheme must match
// e2e/seed/seed.sql.
import http from 'k6/http';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:18080/policy-service/api';
export const RESERVATIONS = `${BASE_URL}/v1/reservations`;
export const WALLETS = 1000;
export const CUSTOM_WALLETS = 500;

export const TRANSACTION_TYPES = ['TRANSFER', 'TRANSFER', 'TRANSFER', 'PAYMENT', 'PAYMENT', 'WITHDRAW'];
export const CHANNELS = ['APP', 'APP', 'WEB', 'ALL'];

const JSON_HEADERS = { 'Content-Type': 'application/json' };

export const reserveLatency = new Trend('reserve_latency', true);
export const releaseLatency = new Trend('release_latency', true);
export const reserved = new Counter('reservations_reserved');
export const rejected = new Counter('reservations_rejected');
export const released = new Counter('reservations_released');
export const unexpected = new Rate('unexpected_responses');

export function wallet(i) {
  const tier = ['T3', 'T1', 'T2'][i % 3];
  return {
    walletId: `e2e-w-${String(i).padStart(4, '0')}`,
    walletType: i % 10 === 0 ? 'MERCHANT' : 'PERSONAL',
    tier,
  };
}

export function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

export function pick(values) {
  return values[Math.floor(Math.random() * values.length)];
}

// Two distinct wallets from [from, to].
export function pair(from = 1, to = WALLETS) {
  const sender = randomInt(from, to);
  let receiver = randomInt(from, to);
  while (receiver === sender) {
    receiver = randomInt(from, to);
  }
  return [wallet(sender), wallet(receiver)];
}

let sequence = 0;

// Unique across VUs, iterations and runs.
export function requestId(prefix = 'k6') {
  sequence += 1;
  return `${prefix}-${__VU}-${sequence}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export function reserveBody({ sender, receiver, amount, transactionType, channel, id }) {
  return {
    requestId: id || requestId(),
    transactionType: transactionType || pick(TRANSACTION_TYPES),
    channel: channel || pick(CHANNELS),
    currency: 'VND',
    amount: String(amount || randomInt(10, 500) * 1000),
    sender,
    receiver,
  };
}

// reserve posts a reservation and records latency and outcome. RESERVED and
// REJECTED are both business successes; anything but 200 is unexpected.
export function reserve(body, tags = {}) {
  const response = http.post(RESERVATIONS, JSON.stringify(body), {
    headers: JSON_HEADERS,
    tags: { name: 'reserve', ...tags },
  });
  reserveLatency.add(response.timings.duration);
  const ok = check(response, {
    'reserve 200': (r) => r.status === 200,
    'reserve has status': (r) => r.status !== 200 || ['RESERVED', 'REJECTED'].includes(r.json('status')),
  });
  unexpected.add(!ok);
  if (response.status === 200) {
    const status = response.json('status');
    if (status === 'RESERVED') reserved.add(1);
    if (status === 'REJECTED') rejected.add(1);
  }
  return response;
}

// release deletes a reservation by its original request id. expected lists
// the acceptable HTTP statuses (409 when racing another release).
export function release(id, expected = [200], tags = {}) {
  const response = http.del(`${RESERVATIONS}/${encodeURIComponent(id)}`, null, {
    tags: { name: 'release', ...tags },
    responseCallback: http.expectedStatuses(...expected),
  });
  releaseLatency.add(response.timings.duration);
  const ok = check(response, { 'release expected status': (r) => expected.includes(r.status) });
  unexpected.add(!ok);
  if (response.status === 200) released.add(1);
  return response;
}

export function summaryPath(name) {
  const dir = __ENV.RESULTS_DIR || 'e2e/results';
  const stamp = new Date().toISOString().replace(/[:.]/g, '-');
  return `${dir}/${name}-${stamp}.json`;
}

// handleSummary writes the full k6 summary JSON next to the text summary.
export function summary(name) {
  return (data) => ({
    [summaryPath(name)]: JSON.stringify(data, null, 2),
    stdout: textSummary(data),
  });
}

function textSummary(data) {
  const metric = (name, field) => {
    const value = data.metrics[name] && data.metrics[name].values[field];
    return value === undefined ? '-' : Number(value).toFixed(2);
  };
  const lines = [
    '',
    `  iterations/s        ${metric('iterations', 'rate')}`,
    `  http_reqs/s         ${metric('http_reqs', 'rate')}`,
    `  dropped_iterations  ${metric('dropped_iterations', 'count')}`,
    `  http_req_failed     ${metric('http_req_failed', 'rate')}`,
    `  unexpected          ${metric('unexpected_responses', 'rate')}`,
    `  reserve p50/p95/p99 ${metric('reserve_latency', 'med')} / ${metric('reserve_latency', 'p(95)')} / ${metric('reserve_latency', 'p(99)')} ms`,
    `  release p50/p95/p99 ${metric('release_latency', 'med')} / ${metric('release_latency', 'p(95)')} / ${metric('release_latency', 'p(99)')} ms`,
    `  reserved/rejected/released ${metric('reservations_reserved', 'count')} / ${metric('reservations_rejected', 'count')} / ${metric('reservations_released', 'count')}`,
    '',
    '  thresholds:',
  ];
  for (const [name, metricData] of Object.entries(data.metrics)) {
    for (const [threshold, result] of Object.entries(metricData.thresholds || {})) {
      lines.push(`    ${result.ok ? 'PASS' : 'FAIL'} ${name} ${threshold}`);
    }
  }
  return `${lines.join('\n')}\n`;
}
