import assert from "node:assert/strict";
import test from "node:test";
import { parseFlatToml, stringifyFlatToml } from "../src/toml.js";

test("parseFlatToml handles Grapes metadata", () => {
  const parsed = parseFlatToml(`
title = 'Define orchestration'
status = 'todo'
priority = 'high'
labels = ['architecture', 'agents']
blocked_by = [3, 5]
created = 2026-04-28T21:17:00Z
`);

  assert.equal(parsed.title, "Define orchestration");
  assert.equal(parsed.status, "todo");
  assert.deepEqual(parsed.labels, ["architecture", "agents"]);
  assert.deepEqual(parsed.blocked_by, [3, 5]);
  assert.equal(parsed.created, "2026-04-28T21:17:00Z");
});

test("stringifyFlatToml omits undefined values", () => {
  assert.equal(
    stringifyFlatToml({
      phase: "claimed",
      attempt: 1,
      missing: undefined,
      labels: ["security", "auth"],
    }),
    "phase = 'claimed'\nattempt = 1\nlabels = ['security', 'auth']\n",
  );
});
