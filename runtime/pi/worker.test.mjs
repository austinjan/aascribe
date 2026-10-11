import test from "node:test";
import assert from "node:assert/strict";
import { parseAnswer, runQuery } from "./worker.mjs";
import { parseCodexAccess } from "./codex-auth.mjs";

test("answer parsing rejects unsupported results and invented citation shapes", () => {
  assert.throws(() => parseAnswer("not json"));
  assert.throws(() => parseAnswer(JSON.stringify({ status: "answered", answer: "Claim", sources: [] })));
  assert.throws(() => parseAnswer(JSON.stringify({ status: "answered", answer: "Claim", sources: [{ path: "x", start_line: 5, end_line: 1 }] })));
  const answer = { status: "insufficient_evidence", answer: "No evidence found.", sources: [] };
  assert.deepEqual(parseAnswer(JSON.stringify(answer)), answer);
});

test("worker refuses remote bridges before creating a model session", async () => {
  await assert.rejects(runQuery({ question: "How?", bridge_url: "https://example.com/", bridge_token: "x".repeat(64) }), /loopback/);
  await assert.rejects(runQuery({ question: " ", bridge_url: "http://127.0.0.1/" }), /question/);
});

test("subscription credentials cannot be used with an unrelated provider", async () => {
  await assert.rejects(runQuery({ question: "How?", bridge_url: "http://127.0.0.1/", bridge_token: "x".repeat(64),
    provider: "google", model: "fixture", auth_file: "must-not-be-opened.json" }), /openai-codex/);
});

test("Codex managed login rejects API keys and detects near-expiry access tokens", () => {
  const now = 1_000_000;
  const token = exp => 'header.' + Buffer.from(JSON.stringify({exp})).toString('base64url') + '.signature';
  assert.throws(() => parseCodexAccess({ auth_mode:'apikey', tokens:{access_token:token(999999)} }, now), /ChatGPT/);
  assert.equal(parseCodexAccess({auth_mode:'chatgpt',tokens:{access_token:token(1100)}}, now).needsRefresh, true);
  const valid = parseCodexAccess({auth_mode:'chatgpt',tokens:{access_token:token(5000),refresh_token:'never-copy'}}, now);
  assert.equal(valid.needsRefresh, false);
  assert.equal('refresh' in valid, false);
});
