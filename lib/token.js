// Makes HS256 JWTs for the k6 scripts (seed.js and load-test.js).
import crypto from 'k6/crypto';
import encoding from 'k6/encoding';

function b64url(text) {
  return encoding.b64encode(text, 'rawurl');
}

export function makeToken(username, secret, ttlSeconds) {
  const ttl = ttlSeconds || 86400;
  const header = b64url(JSON.stringify({ alg: 'HS256', typ: 'JWT' }));
  const payload = b64url(JSON.stringify({ sub: username, exp: Math.floor(Date.now() / 1000) + ttl }));
  const data = header + '.' + payload;
  const signature = crypto.hmac('sha256', secret, data, 'base64rawurl');
  return data + '.' + signature;
}
