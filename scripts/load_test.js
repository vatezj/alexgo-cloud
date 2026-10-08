import http from 'k6/http'
import { check, sleep, group } from 'k6'
import { Trend } from 'k6/metrics'

export const options = {
  stages: [
    { duration: '30s', target: 100 },
    { duration: '2m', target: 800 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<400', 'p(99)<800'],
    http_req_failed: ['rate<0.01'],
    order_create_duration: ['p(95)<500'],
  },
}

const orderCreateTrend = new Trend('order_create_duration')

export function setup() {
  const payload = JSON.stringify({ username: 'admin', password: 'admin123' })
  const res = http.post('http://localhost:8080/api/app/system/auth/login', payload, {
    headers: { 'Content-Type': 'application/json' },
  })
  const ok = check(res, { 'login status 200': (r) => r.status === 200 })
  if (!ok) {
    return { token: '' }
  }
  const body = res.json()
  return { token: body && body.token ? body.token : '' }
}

export default function (data) {
  group('System API', () => {
    const res = http.get('http://localhost:8080/api/admin/system/users', {
      headers: { Authorization: `Bearer ${data.token}` },
    })
    check(res, { 'status < 500': (r) => r.status < 500 })
  })

  group('Order API', () => {
    const payload = JSON.stringify({
      user_id: 1,
      product_id: 1,
      amount: 1,
      status: 1,
      order_no: 'demo',
    })
    const res = http.post('http://localhost:8080/api/admin/order/orders', payload, {
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
    })
    orderCreateTrend.add(res.timings.duration)
    check(res, { 'status < 500': (r) => r.status < 500 })
  })

  sleep(1)
}
