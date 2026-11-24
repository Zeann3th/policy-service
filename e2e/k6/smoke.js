// Suite 01 smoke: one VU, functional assertions against the seeded data.
// Run on a freshly seeded stack (scripts/reset.sh): the waiver assertions
// count transfers from the start of the month.
import { check, group } from 'k6';
import http from 'k6/http';
import { RESERVATIONS, release, requestId, reserve, reserveBody, summary, wallet } from './lib.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1'],
  },
};

const JSON_HEADERS = { 'Content-Type': 'application/json' };

// charge returns what one charge kind (FEE, DISCOUNT, TAX) takes from a
// side ("0" when it has no line); amounts are never negative.
function charge(response, side, kind) {
  const found = (response.json(`${side}.charges`) || []).find((each) => each.kind === kind);
  return found ? found.amount : '0';
}

function post(body) {
  return http.post(RESERVATIONS, JSON.stringify(body), { headers: JSON_HEADERS, responseCallback: http.expectedStatuses(200, 400, 404, 409) });
}

export default function () {
  const prefix = requestId('smoke');
  const merchant = wallet(10); // MERCHANT T1, profile 0 (TIGHT)

  group('waiver makes the first 5 transfers free, then the flat fee + tax applies', () => {
    const sender = wallet(502); // PERSONAL T1, no custom policy
    const fees = [];
    for (let i = 0; i < 6; i++) {
      const r = reserve(reserveBody({ sender, receiver: merchant, amount: 100000, transactionType: 'TRANSFER', channel: 'WEB', id: `${prefix}-waiver-${i}` }));
      fees.push(charge(r, 'sender', 'FEE'), charge(r, 'sender', 'TAX'));
    }
    check(fees, {
      'transfers 1-5 free': (f) => f.slice(0, 10).every((v) => v === '0'),
      '6th transfer pays 1100 fee': (f) => f[10] === '1100',
      '6th transfer pays 10% tax on fee': (f) => f[11] === '110',
    });
  });

  group('payment: percent fee on sender, merchant receiving fee', () => {
    const r = reserve(reserveBody({ sender: wallet(508), receiver: merchant, amount: 200000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(r, {
      'sender 0.5% fee': (x) => charge(x, 'sender', 'FEE') === '1000',
      'sender tax 10% of fee': (x) => charge(x, 'sender', 'TAX') === '100',
      'merchant 1% receiving fee': (x) => charge(x, 'receiver', 'FEE') === '2000',
    });
  });

  group('T3 system discount and PROMO custom discount', () => {
    // wallet 9: PERSONAL T3, profile 4 (no fee overrides): 0.5% fee, 10% T3 discount
    const t3 = reserve(reserveBody({ sender: wallet(9), receiver: wallet(11), amount: 200000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(t3, {
      'T3 fee 0.5%': (x) => charge(x, 'sender', 'FEE') === '1000',
      'T3 system discount 10% of fee': (x) => charge(x, 'sender', 'DISCOUNT') === '100',
      'T3 tax 10% of fee after discount': (x) => charge(x, 'sender', 'TAX') === '90',
    });
    // wallet 3: PERSONAL T3, profile 3 (PROMO: flat 2000 fee, 50% discount)
    for (let i = 0; i < 5; i++) {
      reserve(reserveBody({ sender: wallet(3), receiver: wallet(4), amount: 100000, transactionType: 'TRANSFER', channel: 'WEB' }));
    }
    const promo = reserve(reserveBody({ sender: wallet(3), receiver: wallet(4), amount: 100000, transactionType: 'TRANSFER', channel: 'WEB' }));
    check(promo, {
      'custom fee 2000 overrides 1100': (x) => charge(x, 'sender', 'FEE') === '2000',
      'custom 50% discount': (x) => charge(x, 'sender', 'DISCOUNT') === '1000',
      'tax on fee after discount': (x) => charge(x, 'sender', 'TAX') === '100',
    });
  });

  group('cashback: DISCOUNT on base AMOUNT', () => {
    // wallet 7: PERSONAL T1, profile 2: 1% cashback on PAYMENT
    const r = reserve(reserveBody({ sender: wallet(7), receiver: wallet(8), amount: 200000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(r, {
      'fee 0.5% of amount': (x) => charge(x, 'sender', 'FEE') === '1000',
      'cashback 1% of amount counts as discount': (x) => charge(x, 'sender', 'DISCOUNT') === '2000',
      'tax on NET_FEE ignores cashback': (x) => charge(x, 'sender', 'TAX') === '100',
    });
  });

  group('TIGHT custom limit rejects and adds no usage', () => {
    const sender = wallet(15); // profile 0: daily OUT total 2,000,000
    const big = reserve(reserveBody({ sender, receiver: wallet(16), amount: 2500000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(big, {
      'over custom daily cap is REJECTED': (x) => x.json('status') === 'REJECTED',
      'sender daily total is the violated policy': (x) => (x.json('sender.appliedPolicies') || []).some((each) => each.violated && each.policyType === 'TOTAL_MAX'),
    });
    const fits = reserve(reserveBody({ sender, receiver: wallet(16), amount: 2000000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(fits, { 'rejection consumed nothing: 2,000,000 still fits': (x) => x.json('status') === 'RESERVED' });
  });

  group('expired custom policy is ignored', () => {
    // wallet 50 has an expired TXN_MAX <= 1; it would reject everything
    const r = reserve(reserveBody({ sender: wallet(50), receiver: wallet(51), amount: 100000, transactionType: 'PAYMENT', channel: 'WEB' }));
    check(r, { 'expired override not applied': (x) => x.json('status') === 'RESERVED' });
  });

  group('release lifecycle', () => {
    const id = `${prefix}-release`;
    const r = reserve(reserveBody({ sender: wallet(601), receiver: wallet(602), amount: 50000, id }));
    check(r, { 'reserved': (x) => x.json('status') === 'RESERVED' });
    const first = release(id, [200]);
    check(first, { 'released': (x) => x.json('status') === 'RELEASED' });
    const second = release(id, [200]);
    check(second, { 'second release returns the first response': (x) => x.body === first.body });
    release(`${prefix}-missing`, [404]);
  });

  group('validation', () => {
    const base = reserveBody({ sender: wallet(701), receiver: wallet(702), amount: 1000 });
    check(post({ ...base, amount: '0' }), { 'amount 0 is 400': (x) => x.status === 400 });
    check(post({ ...base, amount: '1.00001' }), { 'more than 4 decimals is 400': (x) => x.status === 400 });
    check(post({ ...base, receiver: base.sender }), { 'same wallet is 400': (x) => x.status === 400 });
    check(post({ ...base, sender: { walletId: 'nobody', walletType: 'GHOST', tier: 'T9' } }), { 'no policy is 400': (x) => x.status === 400 });
    const once = post(base);
    check(post(base), { 'duplicate requestId is 409': (x) => once.status === 200 && x.status === 409 });
  });
}

export const handleSummary = summary('smoke');
