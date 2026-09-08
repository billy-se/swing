import ws from 'k6/ws';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 1000 }, // Ramp up to 400 VUs over 10 seconds
    { duration: '2m', target: 1000 },  // Hold steady at 400 VUs for 2 minutes
    { duration: '10s', target: 0 },   // Ramp down to 0 VUs
  ],
};

export default function () {
  // Added the token query parameter required by your Go backend
  const url = 'ws://localhost:2026/ws?token=load-test-token';

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', function () {
      socket.send(JSON.stringify({ event: 'ping', user: `vu-${__VU}` }));
    });

    socket.on('message', function (message) {
      // Handle incoming broadcast messages
    });

    socket.on('close', function () {
      // Graceful close
    });

    sleep(10);
  });

  check(res, { 'status is 101': (r) => r && r.status === 101 });
}