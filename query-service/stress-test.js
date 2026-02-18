import http from 'k6/http';
import { check } from 'k6';

export const options = {
    stages: [
        { duration: '30s', target: 1000 },
        { duration: '1m', target: 10000 },   // 10K users
        { duration: '2m', target: 50000 },   // 50K peak
        { duration: '30s', target: 0 },
    ],
};

export default function () {
    // Use user IDs from 1000-100999
    const userId = 1000 + (__VU % 100000);

    const payload = JSON.stringify({
        match_id: 3,        // Match 3 has 1000 VIP seats
        category: 'VIP',
        user_id: userId,
        quantity: 1,
    });

    const res = http.post('http://api.localhost/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
    });

    check(res, {
        'status OK': (r) => r.status === 200 || r.status === 202,
    });
}