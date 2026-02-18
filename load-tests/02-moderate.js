import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const requestCount = new Counter('requests_total');
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');

export const options = {
    stages: [
        { duration: '30s', target: 1000 },
        { duration: '1m', target: 5000 },
        { duration: '2m', target: 10000 },
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        'request_duration': ['p(95)<2000'],
        'errors': ['rate<0.1'],
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
    const res = http.post('http://api.localhost/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
    });
    requestDuration.add(Date.now() - start);

    const success = check(res, {
        'status OK': (r) => r.status === 200 || r.status === 202,
    });

    if (!success) {
        errorRate.add(1);
    }

    sleep(Math.random() * 0.5);
}