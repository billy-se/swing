import ws from 'k6/ws';
import { check } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 1250 },  // Quick ramp up to 1,250 VUs
    { duration: '20s', target: 1250 },  // Hold steady
    { duration: '10s', target: 0 },     // Quick ramp down (Total: 40s)
  ],
};

export default function () {
  const url = 'ws://localhost:2026/ws?ticket=some-valid-ticket';

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', function () {
      socket.send(JSON.stringify({ event: 'ping', user: `vu-${__VU}` }));
      
      socket.setTimeout(function () {
        socket.close();
      }, 2000);
    });
  });

  check(res, {
    'status is 101 switching protocols': (r) => r && r.status === 101,
  });
}