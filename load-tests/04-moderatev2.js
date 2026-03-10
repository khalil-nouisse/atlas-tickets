import http from 'k6/http';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const requestCount = new Counter('requests_total');
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');

// All available ticket categories — spread load across them all
const TARGETS = [
    { match_id: 1, category: 'VIP' },
    { match_id: 1, category: 'CAT1' },
    { match_id: 2, category: 'VIP' },
    { match_id: 2, category: 'CAT1' },
    { match_id: 3, category: 'VIP' },
    { match_id: 3, category: 'CAT1' },
];

export const options = {
    stages: [
        { duration: '20s', target: 500 },  // Quick ramp
        { duration: '30s', target: 3000 },  // Aggressive ramp to peak
        { duration: '2m', target: 3000 },  // Sustain — long enough for Grafana to capture workers
        { duration: '20s', target: 0 },  // Ramp down
    ],
    thresholds: {
        'request_duration': ['p(95)<2000'],
        'errors': ['rate<0.05'],
    },
};

export default function () {
    // Each VU picks a ticket category deterministically
    const target = TARGETS[__VU % TARGETS.length];
    const userId = 1000 + (__VU % 100000);

    const payload = JSON.stringify({
        match_id: target.match_id,
        category: target.category,
        user_id: userId,
        quantity: 1,
    });

    requestCount.add(1);

    const start = Date.now();
    const res = http.post('http://api.84.8.216.45.nip.io/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
        timeout: '10s',
    });
    requestDuration.add(Date.now() - start);

    const success = check(res, {
        'status 202': (r) => r.status === 202,
    });

    if (!success) errorRate.add(1);

    // No sleep — fire requests as fast as possible to maximise
    // concurrent goroutines in the worker pool
}