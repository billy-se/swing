import ws from 'k6/ws';
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  setupTimeout: '4m',
  scenarios: {
    target_ceiling_test: {
      executor: 'ramping-vus',
      startVUs: 5,
      stages: [
        { duration: '45s', target: 100 },
        { duration: '1m', target: 250 },  
        { duration: '45s', target: 0 },
      ]
    },
  },
  thresholds: {
    'http_req_failed': ['rate<0.05'], 
  },
};

const BASE_URL = 'http://localhost:2026';
const RUN_ID = Date.now();
const USER_POOL_SIZE = 100;

export function setup() {
  console.log(`Pre-seeding ${USER_POOL_SIZE} users safely...`);
  const headers = { 'Content-Type': 'application/json' };

  for (let i = 1; i <= USER_POOL_SIZE; i++) {
    const res = http.post(`${BASE_URL}/api/register`, JSON.stringify({
      email: `user_${RUN_ID}_${i}@example.com`,
      password: 'TestPassword123!',
      username: `u_${RUN_ID}_${i}`
    }), { headers, timeout: '5s' });

    if (i % 10 === 0) {
      sleep(0.05);
    }
  }

  console.log('Logging in setup user for initial argument seeding...');
  const loginRes = http.post(`${BASE_URL}/api/login`, JSON.stringify({
    email: `user_${RUN_ID}_1@example.com`,
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
  } else {
    console.log('Setup login failed with status:', loginRes.status);
  }

  return { runId: RUN_ID };
}

const vuSessions = {};

export default function (data) {
  const headers = { 'Content-Type': 'application/json' };

  if (!vuSessions[__VU]) {
    const userId = ((__VU - 1) % USER_POOL_SIZE) + 1; 
    const email = `user_${data.runId}_${userId}@example.com`;
    const password = 'TestPassword123!';

    const loginRes = http.post(`${BASE_URL}/api/login`, JSON.stringify({
      email: email,
      password: password
    }), { headers, timeout: '5s' });

    if (!check(loginRes, { 'logged in': (r) => r.status === 200 })) {
      return;
    }

    try {
      const resBody = JSON.parse(loginRes.body);
      vuSessions[__VU] = {
        token: resBody.access_token,
        userId: userId
      };
    } catch (e) {
      return;
    }
  }

  const session = vuSessions[__VU];
  const authHeaders = { 
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${session.token}`
  };

  const ticketRes = http.post(`${BASE_URL}/api/ws-ticket`, null, { headers: authHeaders, timeout: '3s' });
  if (!ticketRes || ticketRes.status !== 200) return;

  let ticketData;
  try {
    ticketData = JSON.parse(ticketRes.body);
  } catch (e) {
    return;
  }
  const ticket = ticketData.ticket || ticketData.ws_ticket;
  if (!ticket) return;

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
  let targetArgumentId = 1;

  if (check(getArgsRes, { 'fetched arguments successfully': (r) => r.status === 200 })) {
    try {
      const args = JSON.parse(getArgsRes.body);
      if (Array.isArray(args) && args.length > 0) {
        const index = (__VU + __ITER) % args.length;
        targetArgumentId = args[index].id;
      }
    } catch (e) {}
  }

  const getCommentsRes = http.get(`${BASE_URL}/api/comments?argument_id=${targetArgumentId}`, { headers: authHeaders, timeout: '3s' });
  check(getCommentsRes, { 'fetched comments successfully': (r) => r.status === 200 });

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

  sleep(1);
}