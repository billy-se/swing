import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  setupTimeout: '4m',
  scenarios: {
    target_ceiling_test: {
      executor: 'ramping-vus',
      startVUs: 2,
      stages: [
        { duration: '30s', target: 8 },   // Gentle warmup
        { duration: '1m', target: 15 },   // Safe peak load
        { duration: '30s', target: 0 },   // Cool down
      ]
    },
  },
  thresholds: {
    'http_req_failed': ['rate<0.05'], 
  },
};

const BASE_URL = 'http://localhost:2026';
const RUN_ID = Date.now();
const USER_POOL_SIZE = 30;

export function setup() {
  console.log(`Pre-seeding ${USER_POOL_SIZE} users safely...`);
  const headers = { 'Content-Type': 'application/json' };

  const seedEmail = `seed_admin_${RUN_ID}@example.com`;
  const seedReg = http.post(`${BASE_URL}/api/register`, JSON.stringify({
    email: seedEmail,
    password: 'TestPassword123!',
    username: `admin_${RUN_ID}`
  }), { headers, timeout: '5s' });
  
  if (seedReg.status !== 201 && seedReg.status !== 200) {
    console.log(`Warning: Seed admin registration returned status ${seedReg.status}`);
  }

  let failedRegistrations = 0;
  for (let i = 1; i <= USER_POOL_SIZE; i++) {
    const regRes = http.post(`${BASE_URL}/api/register`, JSON.stringify({
      email: `user_${RUN_ID}_${i}@example.com`,
      password: 'TestPassword123!',
      username: `u_${RUN_ID}_${i}`
    }), { headers, timeout: '5s' });

    if (regRes.status !== 201 && regRes.status !== 200) {
      failedRegistrations++;
    }

    if (i % 10 === 0) {
      sleep(0.05);
    }
  }

  const loginRes = http.post(`${BASE_URL}/api/login`, JSON.stringify({
    email: seedEmail,
    password: 'TestPassword123!'
  }), { headers, timeout: '5s' });

  if (loginRes.status === 200) {
    try {
      const token = JSON.parse(loginRes.body).access_token;
      const authHeaders = { 'Content-Type': 'application/json', 'Authorization': `Bearer ${token}` };

      for (let j = 1; j <= 3; j++) {
        http.post(`${BASE_URL}/api/arguments`, JSON.stringify({
          title: `Seed Argument ${j} - ${RUN_ID}`,
          content: `Initial content for load testing argument ${j}.`
        }), { headers: authHeaders, timeout: '3s' });
      }
      console.log('Initial seed arguments created successfully.');
    } catch (e) {
      console.log('Failed to seed arguments:', e);
    }
  }

  return { runId: RUN_ID };
}

const vuSessions = {};
const failedLogins = {};

export default function (data) {
  const headers = { 'Content-Type': 'application/json' };

  if (!vuSessions[__VU]) {
    if (failedLogins[__VU]) {
      sleep(1);
      return;
    }

    const userId = ((__VU - 1) % USER_POOL_SIZE) + 1; 
    const email = `user_${data.runId}_${userId}@example.com`;
    const password = 'TestPassword123!';

    const loginRes = http.post(`${BASE_URL}/api/login`, JSON.stringify({
      email: email,
      password: password
    }), { headers, timeout: '5s' });

    if (!check(loginRes, { 'logged in': (r) => r.status === 200 })) {
      failedLogins[__VU] = true;
      sleep(1);
      return;
    }

    try {
      const resBody = JSON.parse(loginRes.body);
      vuSessions[__VU] = {
        token: resBody.access_token,
        userId: userId
      };
    } catch (e) {
      failedLogins[__VU] = true;
      sleep(1);
      return;
    }
  }

  const session = vuSessions[__VU];
  const authHeaders = { 
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${session.token}`
  };

  const ticketRes = http.post(`${BASE_URL}/api/ws-ticket`, null, { headers: authHeaders, timeout: '3s' });
  if (!ticketRes || ticketRes.status !== 200) {
    sleep(1);
    return;
  }

  let ticketData;
  try {
    ticketData = JSON.parse(ticketRes.body);
  } catch (e) {
    sleep(1);
    return;
  }
  const ticket = ticketData.ticket || ticketData.ws_ticket;
  if (!ticket) {
    sleep(1);
    return;
  }

  const wsUrl = `ws://localhost:2026/ws?ticket=${ticket}`;
  ws.connect(wsUrl, {}, function (socket) {
    socket.on('open', function () {
      socket.setTimeout(function () {
        socket.close();
      }, 300);
    });
  });

  const newArgRes = http.post(`${BASE_URL}/api/arguments`, JSON.stringify({
    title: `Argument by VU ${__VU} Iter ${__ITER}`,
    content: 'Dynamic load test content.'
  }), { headers: authHeaders, timeout: '3s' });
  
  check(newArgRes, { 'created argument successfully': (r) => r.status === 201 || r.status === 200 });

  const getArgsRes = http.get(`${BASE_URL}/api/arguments/top`, { headers: authHeaders, timeout: '3s' });
  let targetArgumentId = null;

  if (check(getArgsRes, { 'fetched arguments successfully': (r) => r.status === 200 })) {
    try {
      const args = JSON.parse(getArgsRes.body);
      if (Array.isArray(args) && args.length > 0) {
        const index = (__VU + __ITER) % args.length;
        targetArgumentId = args[index].id;
      }
    } catch (e) {}
  }

  if (targetArgumentId) {
    if ((__VU + __ITER) % 3 === 0) {
      const getCommentsRes = http.get(`${BASE_URL}/api/comments?argument_id=${targetArgumentId}`, { headers: authHeaders, timeout: '5s' });
      check(getCommentsRes, { 'fetched comments successfully': (r) => r.status === 200 });
    }

    const commentPayload = JSON.stringify({
      argument_id: targetArgumentId,
      parent_id: null,
      content: `Comment from VU ${__VU} iteration ${__ITER}`
    });
    
    const postCommentRes = http.post(`${BASE_URL}/api/comments`, commentPayload, { headers: authHeaders, timeout: '3s' });
    check(postCommentRes, { 'posted comment successfully': (r) => r.status === 201 || r.status === 200 });

    let createdCommentId = 1;
    if (postCommentRes.status === 201 || postCommentRes.status === 200) {
      try {
        const cBody = JSON.parse(postCommentRes.body);
        if (cBody.id) createdCommentId = cBody.id;
      } catch (e) {}
    }

    const firePayload = JSON.stringify({
      comment_id: createdCommentId
    });
    const fireRes = http.post(`${BASE_URL}/api/comments/fire`, firePayload, { headers: authHeaders, timeout: '3s' });
    check(fireRes, { 'fire reaction processed': (r) => r.status === 200 || r.status === 400 });
  }

  sleep(4);
}