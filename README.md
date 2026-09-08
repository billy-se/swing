Swing - A synchronized discussion platform built in Go, using mux for routing and WebSockets for handling concurrent user connections with real-time comment thread updates

Why I Build It? to explore, solve and properly scailing bottlenecks real-time collaboration platform. Proving that Go can handle heavy concurrent users

<table>
  <thead>
    <tr>
      <th>Test Scenario</th>
      <th>Virtual Users (VUs)</th>
      <th>Handshake Latency (avg)</th>
      <th>Success Rate</th>
      <th>Bottleneck / Observation</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><strong>Baseline Test</strong></td>
      <td>10 VUs</td>
      <td>3.34 ms</td>
      <td>100.0%</td>
      <td>Clean connection</td>
    </tr>
    <tr>
      <td><strong>Stress Test</strong></td>
      <td>500 VUs</td>
      <td>239.65 ms</td>
      <td>0.0% (Client-side teardown)</td>
      <td>User requested disconnects</td>
    </tr>
  </tbody>
</table>

<table>
  <tr>
    <td align="center">
      <img src="./assets/baseLineTest.png" alt="Baseline" width="100%">
      <br>
      <b>Baseline Test</b>
    </td>
    <td align="center">
      <img src="./assets/stressTest.png" alt="Stress Test" width="100%">
      <br>
      <b>Stress Test</b>
    </td>
  </tr>
</table>

Baseline Test Summary: Document steady-state performance—such as nominal request throughput, P95/P99 latency under standard load, and a 0.0% error rate with user-requested disconnects.

Stress Test Summary: Outline the peak concurrency limits reached before hitting local socket constraints, proving how the system degrades gracefully under heavy pressure.
