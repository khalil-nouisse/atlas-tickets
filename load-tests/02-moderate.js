import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const requestCount = new Counter('requests_total');
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');

export const options = {
    stages: [
        { duration: '30s', target: 200 },  // Smooth ramp-up
        { duration: '1m', target: 1200 }, // Peak: slightly higher to test the new 1.0 CPU limit
        { duration: '1m', target: 1200 }, // Sustain peak to show worker stacking
        { duration: '30s', target: 0 },    // Graceful ramp-down
    ],
    thresholds: {
        // With 1.0 CPU, p(95) under 1.5s is a very strong professional result
        'request_duration': ['p(95)<1500'],
        'errors': ['rate<0.01'],
    },
};

export default function () {
    const userId = 1000 + (__VU % 100000);

    const payload = JSON.stringify({
        match_id: 3,
        category: 'VIP',
        user_id: userId,
        quantity: 1,
    });

    requestCount.add(1);

    const start = Date.now();
    const res = http.post('http://api.84.8.216.45.nip.io/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
        timeout: '10s', // Prevent k6 from hanging if the network stutters
    });
    requestDuration.add(Date.now() - start);

    const success = check(res, {
        'status OK': (r) => r.status === 200 || r.status === 202,
    });

    if (!success) {
        errorRate.add(1);
    }

    // Increased sleep slightly (0.8s) to simulate more realistic "human" clicking
    // This helps maintain a high throughput without triggering a TCP connection storm
    sleep(Math.random() * 0.8);
}