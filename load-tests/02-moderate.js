import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const requestCount = new Counter('requests_total');
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration');

export const options = {
    stages: [
        { duration: '30s', target: 200 },
        { duration: '1m', target: 1000 }, 
        { duration: '1m', target: 1000 }, 
        { duration: '30s', target: 0 },
    ],
    thresholds: {
        'request_duration': ['p(95)<1000'], // Goal: 95% under 1 second
        'errors': ['rate<0.01'],            // Goal: Less than 1% errors
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
    // Pointing to your ingress
    const res = http.post('http://api.84.8.216.45.nip.io/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
    });
    requestDuration.add(Date.now() - start);

    const success = check(res, {
        'status OK': (r) => r.status === 200 || r.status === 202,
    });

    if (!success) {
        errorRate.add(1);
    }

    // A tiny randomized sleep simulates real users clicking buttons
    sleep(Math.random() * 0.5); 
}