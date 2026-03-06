import http from 'k6/http';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const purchaseAttempts = new Counter('purchase_attempts');
const successfulPurchases = new Counter('successful_purchases');
const soldOutResponses = new Counter('sold_out_responses');
const errorRate = new Rate('errors');
const requestDuration = new Trend('request_duration_ms');

export const options = {
    stages: [
        { duration: '1m', target: 10000 },
        { duration: '2m', target: 30000 },
        { duration: '3m', target: 50000 },  // Peak
        { duration: '2m', target: 30000 },
        { duration: '1m', target: 0 },
    ],
    thresholds: {
        'request_duration_ms': ['p(95)<3000', 'p(99)<5000'],
        'errors': ['rate<0.05'],
    },
};

export default function () {
    purchaseAttempts.add(1);

    const userId = 1000 + (__VU % 100000);

    const payload = JSON.stringify({
        match_id: 3,
        category: 'VIP',
        user_id: userId,
        quantity: Math.floor(Math.random() * 2) + 1,  // 1-2 tickets
    });

    const start = Date.now();

    //          local :  http://api.localhost/api/tickets
    const res = http.post('http://api.84.8.216.45.nip.io/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
    });
    const duration = Date.now() - start;
    requestDuration.add(duration);

    const success = check(res, {
        'status is 2xx': (r) => r.status >= 200 && r.status < 300,
    });

    if (success) {
        if (res.status === 200 || res.status === 202) {
            successfulPurchases.add(1);
        }
        // Check body for SOLD_OUT
        try {
            const body = JSON.parse(res.body);
            if (body.status === 'SOLD_OUT' || body.message?.includes('SOLD OUT')) {
                soldOutResponses.add(1);
            }
        } catch (e) {
            // Body not JSON
        }
    } else {
        errorRate.add(1);
    }
}