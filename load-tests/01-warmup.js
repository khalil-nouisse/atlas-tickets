import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    vus: 100,
    duration: '30s',
};

export default function () {
    const userId = 1000 + (__VU % 1000);

    const payload = JSON.stringify({
        match_id: 3,
        category: 'VIP',
        user_id: userId,
        quantity: 1,
    });

    //          local :  http://api.localhost/api/tickets
    const res = http.post('http://api.84.8.216.45.nip.io/api/tickets', payload, {
        headers: { 'Content-Type': 'application/json' },
    });

    check(res, {
        'status OK': (r) => r.status === 200 || r.status === 202,
    });

    sleep(0.5);
}