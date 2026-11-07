// ============================================================
// one-core-challenge - load test
//
// Load level: one passing level per run (fixed steps of +50 users).
// A run passes if it holds the level for 5 minutes with:
//   - under 1% errors (timeouts, resets, 5xx, schema/status violations)
//   - 95% of replies under 500 ms
// k6 exit code: 0 = level passed, anything else = level failed.
// Threshold breaches abort the run early, so a dead level costs no time.
//
// Usage:  k6 run -e TARGET=<users> load-test.js
// Smoke:  k6 run -e TARGET=10 -e DURATION=30s load-test.js
// Record: k6 run -e TARGET=<users> --summary-export results/level_<users>.json load-test.js
//
// Env:    TARGET    simulated users for this level (default 300)
//         DURATION  how long the level holds (default 5m)
//         RAMP      time to spin VUs up before the hold (default 60s per rules.md)
//         BASE      app base URL (default http://localhost:8080)
//         JWT_SECRET HMAC secret for JWT tokens (default one-core-challenge-secret)
// ============================================================

import http from 'k6/http';
import { check, sleep } from 'k6';
import exec from 'k6/execution';
import { makeToken } from './lib/token.js';

const BASE = __ENV.BASE || 'http://localhost:8080';
const SECRET = __ENV.JWT_SECRET || 'one-core-challenge-secret';
const TARGET = __ENV.TARGET ? Number(__ENV.TARGET) : 300;
const DURATION = __ENV.DURATION || '5m';
const RAMP = __ENV.RAMP || '60s';

const JSON_HDRS = { 'Content-Type': 'application/json' };
const thinkSeconds = () => 2 + Math.random() * 3;

export const options = {
  scenarios: {
    challenge: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: RAMP, target: TARGET },
        { duration: DURATION, target: TARGET },
      ],
      gracefulRampDown: '0s',
    },
  },
  thresholds: {
    // Errors: failed requests count timeouts, resets, 5xx, and check failures
    http_req_failed: [{ threshold: 'rate<0.01', abortOnFail: true }],
    checks: [{ threshold: 'rate>0.99', abortOnFail: true }],
    http_req_duration: [{ threshold: 'p(95)<500', abortOnFail: true }],
  },
};

function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}

export function setup() {
  // Verify server is reachable and seed data is present
  const feedRes = http.get(`${BASE}/feed`);
  if (feedRes.status !== 200) {
    throw new Error(`server unreachable: GET /feed -> ${feedRes.status}`);
  }

  const posts = feedRes.json('posts');
  if (!posts || posts.length === 0) {
    throw new Error('database is not seeded. Run seed.js before starting load-test.js');
  }

  // Pre-generate tokens for up to 1,000 active users (matching seed data user0000-user0999)
  const tokens = [];
  for (let i = 0; i < 1000; i++) {
    const username = 'user' + String(i).padStart(4, '0');
    tokens.push({
      username,
      token: makeToken(username, SECRET, 86400),
    });
  }

  return { tokens };
}

export default function (data) {
  // Simulated user assignment (user0000 - user0999)
  const userObj = data.tokens[(exec.vu.idInTest - 1) % data.tokens.length];
  const userHeaders = authHeaders(userObj.token);

  // 1. GET /feed
  const feedRes = http.get(`${BASE}/feed`, { tags: { name: 'GET /feed' } });
  const feedOk = check(feedRes, {
    'feed status is 200': (r) => r.status === 200,
    'feed has posts array': (r) => Array.isArray(r.json('posts')) && r.json('posts').length > 0,
  });
  sleep(thinkSeconds());

  // 2. GET /posts/:id (select post from feed or seed range 1..10000)
  let postId = 1;
  if (feedOk) {
    const posts = feedRes.json('posts');
    postId = posts[Math.floor(Math.random() * posts.length)].id;
  } else {
    postId = 1 + Math.floor(Math.random() * 10000);
  }

  const postRes = http.get(`${BASE}/posts/${postId}`, { tags: { name: 'GET /posts/:id' } });
  check(postRes, {
    'post status is 200': (r) => r.status === 200,
    'post id matches': (r) => r.json('post.id') === postId,
  });
  sleep(thinkSeconds());

  // 3. One write: like or comment (50/50)
  if (Math.random() < 0.5) {
    const likeRes = http.post(`${BASE}/posts/${postId}/like`, null, Object.assign({}, userHeaders, {
      tags: { name: 'POST /posts/:id/like' },
    }));
    // 201 if first like by user, 200 if duplicate like
    check(likeRes, {
      'like status is 200 or 201': (r) => r.status === 201 || r.status === 200,
      'like response has post_id': (r) => r.json('post_id') === postId,
    });
  } else {
    const commentRes = http.post(
      `${BASE}/posts/${postId}/comments`,
      JSON.stringify({ content: 'benchmark load test comment' }),
      Object.assign({}, userHeaders, { tags: { name: 'POST /posts/:id/comments' } })
    );
    check(commentRes, {
      'comment status is 201': (r) => r.status === 201,
      'comment response has post_id': (r) => r.json('comment.post_id') === postId,
    });
  }
  sleep(thinkSeconds());

  // 4. GET /users/:username (Profile visit and counted write)
  const targetUser = data.tokens[Math.floor(Math.random() * data.tokens.length)].username;
  const profRes = http.get(`${BASE}/users/${targetUser}`, { tags: { name: 'GET /users/:username' } });
  check(profRes, {
    'profile status is 200': (r) => r.status === 200,
    'profile username matches': (r) => r.json('user.username') === targetUser,
  });
  sleep(thinkSeconds());
}
