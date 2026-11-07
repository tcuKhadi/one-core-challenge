// Loads the seed data through the public API.
// Usage:
//   k6 run -e BASE_URL=http://localhost:8080 -e JWT_SECRET=one-core-challenge-secret seed.js
//
// Result on a fresh database: 1,000 users, 10,000 posts, 30,000 comments, 100,000 likes.
import http from 'k6/http';
import exec from 'k6/execution';
import { check } from 'k6';
import { makeToken } from './lib/token.js';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const SECRET = __ENV.JWT_SECRET || 'one-core-challenge-secret';

const USERS = 1000;
const POSTS = 10000;
const COMMENTS_PER_POST = 3;
const LIKES_PER_POST = 10;

export const options = {
  scenarios: {
    seed: { executor: 'shared-iterations', vus: 50, iterations: POSTS, maxDuration: '30m' },
  },
  thresholds: {
    checks: [{ threshold: 'rate==1', abortOnFail: true }],
  },
};

function username(n) {
  return 'user' + String(n).padStart(4, '0');
}

function headers(token, tag) {
  return {
    headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
    tags: { name: tag },
  };
}

export function setup() {
  const tokens = [];
  for (let u = 0; u < USERS; u++) {
    tokens.push(makeToken(username(u), SECRET, 86400));
  }
  return tokens;
}

export default function (tokens) {
  const i = exec.scenario.iterationInTest;

  const created = http.post(
    BASE + '/posts',
    JSON.stringify({ content: 'seed post ' + i }),
    headers(tokens[i % USERS], 'POST /posts')
  );
  if (!check(created, { 'post created (201)': (r) => r.status === 201 })) {
    return;
  }
  const id = created.json('post.id');

  for (let k = 0; k < COMMENTS_PER_POST; k++) {
    const author = (i * 13 + k * 7) % USERS;
    const r = http.post(
      BASE + '/posts/' + id + '/comments',
      JSON.stringify({ content: 'seed comment ' + i + '-' + k }),
      headers(tokens[author], 'POST /posts/:id/comments')
    );
    check(r, { 'comment created (201)': (x) => x.status === 201 });
  }

  for (let j = 0; j < LIKES_PER_POST; j++) {
    const liker = (i + j * 97) % USERS;
    const r = http.post(
      BASE + '/posts/' + id + '/like',
      null,
      headers(tokens[liker], 'POST /posts/:id/like')
    );
    check(r, { 'like created (201)': (x) => x.status === 201 });
  }
}
