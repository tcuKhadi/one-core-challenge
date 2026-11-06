// ============================================================
// one-core-challenge - load test
//
// Load level: one passing level per run (fixed steps of +50 users).
// A run passes if it holds the level for 5 minutes with:
//   - under 1% errors (timeouts, resets, 5xx)
//   - 95% of replies under 500 ms
// k6 exit code: 0 = level passed, anything else = level failed.
// Threshold breaches abort the run early, so a dead level costs no time.
//
// Usage:  k6 run -e TARGET=<users> load-test.js
// Smoke:  k6 run -e TARGET=10 -e DURATION=30s load-test.js
// Record: k6 run -e TARGET=<users> --summary-export results/level_<users>.json load-test.js
//         (keep the JSONs + exit codes, that is the score table per level)
//
// Env:   TARGET  simulated users for this level (default 300)
//        DURATION how long the level holds (default 5m)
//        RAMP    time to spin VUs up before the hold (default 30s)
//        BASE    app base URL (default http://localhost:8080)
//
// One simulated user (rules.md): GET /feed -> GET /posts/:id ->
// one write (POST /posts/:id/like or POST /posts/:id/comments) ->
// GET /users/:username, pausing 2-5 s between actions.
// ============================================================

import http from 'k6/http';
import { check, sleep } from 'k6';
import exec from 'k6/execution';

const BASE = __ENV.BASE || 'http://localhost:8080';
const TARGET = __ENV.TARGET ? Number(__ENV.TARGET) : 300;
const DURATION = __ENV.DURATION || '5m';
const RAMP = __ENV.RAMP || '30s';
const SEEDS = 20; // seed users/posts so every user has things to read and like

const JSON_HDRS = { 'Content-Type': 'application/json' };
const thinkSeconds = () => 2 + Math.random() * 3;

export const options = {
  scenarios: {
    challenge: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: RAMP, target: TARGET },      // reach the level fast, ramp does not count
        { duration: DURATION, target: TARGET },  // hold, this is what is scored
      ],
      gracefulRampDown: '0s',
    },
  },
  thresholds: {
    // Errors: failed requests count timeouts, resets, and 5xx, per rules.md
    http_req_failed: [{ threshold: 'rate<0.01', abortOnFail: true }],
    http_req_duration: [{ threshold: 'p(95)<500', abortOnFail: true }],
  },
};

// Run setup on the load machine, against a cold-started server.
// Seeds 20 posts so every simulated user has content immediately.
export function setup() {
  const up = http.get(`${BASE}/feed`);
  if (up.status !== 200) {
    throw new Error(`server did not start answering: GET /feed -> ${up.status}`);
  }

  const ids = [];
  for (let i = 1; i <= SEEDS; i++) {
    const res = http.post(
      `${BASE}/posts`,
      JSON.stringify({ username: `seed_${i}`, content: `benchmark post ${i}` }),
      { headers: JSON_HDRS }
    );
    const ok = check(res, { 'seed post created': (r) => r.status === 201 });
    if (ok) ids.push(res.json().id);
  }
  if (ids.length < SEEDS) {
    throw new Error(`seeding failed: created ${ids.length}/${SEEDS} posts`);
  }
  return { ids };
}

export default function (data) {
  // 1. GET /feed (newest posts)
  const feedRes = http.get(`${BASE}/feed`, { tags: { name: 'GET /feed' } });
  check(feedRes, { 'feed is 200': (r) => r.status === 200 });
  sleep(thinkSeconds());

  // 2. GET /posts/:id (one post with counts)
  // Always a known-good seed id: the feed list would work too, but a fixed
  // id keeps every user hitting identical work at every level.
  const id = data.ids[Math.floor(Math.random() * data.ids.length)];
  const postRes = http.get(`${BASE}/posts/${id}`, { tags: { name: 'GET /posts/:id' } });
  check(postRes, { 'post is 200': (r) => r.status === 200 });
  sleep(thinkSeconds());

  // 3. one durable write: like or comment, 50/50
  const username = `seed_${1 + (exec.vu.idInTest % SEEDS)}`;
  if (Math.random() < 0.5) {
    const likeRes = http.post(`${BASE}/posts/${id}/like`, null, { tags: { name: 'POST like' } });
    check(likeRes, { 'like is 201': (r) => r.status === 201 });
  } else {
    const comRes = http.post(
      `${BASE}/posts/${id}/comments`,
      JSON.stringify({ username, content: 'load test comment' }),
      { headers: JSON_HDRS, tags: { name: 'POST comment' } }
    );
    check(comRes, { 'comment is 201': (r) => r.status === 201 });
  }
  sleep(thinkSeconds());

  // 4. GET /users/:username - a counted profile visit, and a durable write
  const profRes = http.get(`${BASE}/users/${username}`, { tags: { name: 'GET /users/:username' } });
  check(profRes, { 'profile is 200': (r) => r.status === 200 });
  sleep(thinkSeconds()); // end-of-loop pause, then the VU starts again
}
