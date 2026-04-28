import assert from "node:assert/strict";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createEventBus } from "../src/event-bus.js";
import { DefaultGrapesStore } from "../src/grapes-store.js";
import { OrchestratorRuntime } from "../src/orchestrator.js";
import { DefaultPolicy } from "../src/policy.js";
import { DefaultWorkflowLoader } from "../src/workflow-loader.js";
import type {
  AgentRunner,
  DispatchContext,
  FileWatcher,
  GrapesIssue,
  RunnerResult,
  Verifier,
  VerificationDecision,
  WorktreeInfo,
  WorktreeManager,
} from "../src/types.js";

test("orchestrator dispatches todo issue and review gate overrides done", async () => {
  const cwd = await mkdtemp(join(tmpdir(), "vineyard-"));
  await writeIssue(cwd, {
    status: "todo",
    labels: ["security"],
  });
  await writeFile(join(cwd, "WORKFLOW.md"), "Run tests and write the implementation artifact.\n", "utf8");

  const store = new DefaultGrapesStore({ cwd, grapesCommand: "" });
  const eventBus = createEventBus();
  const statusChanged = once(eventBus, "status_changed");

  const runtime = new OrchestratorRuntime({
    cwd,
    orchestratorId: "test",
    grapesStore: store,
    workflowLoader: new DefaultWorkflowLoader({ cwd }),
    worktreeManager: new FakeWorktreeManager(cwd),
    runner: new ArtifactRunner(),
    verifier: new PassingVerifier(),
    policy: new DefaultPolicy(),
    watcher: new NoopWatcher(),
    eventBus,
    autoStartWatcher: false,
  });

  await runtime.reconcile("test");
  await statusChanged;

  const issue = await store.getIssue(1);
  assert.equal(issue?.status, "review");

  const claim = await store.readClaim(1);
  assert.equal(claim?.phase, "complete");

  const review = await readFile(join(cwd, ".grapes", "1", "review.md"), "utf8");
  assert.match(review, /Status: review/);
  assert.match(review, /Human review is required/);
});

test("policy blocks issues with unfinished blockers", async () => {
  const cwd = await mkdtemp(join(tmpdir(), "vineyard-"));
  await writeIssue(cwd, { id: 1, status: "todo", blockedBy: [2] });
  await writeIssue(cwd, { id: 2, status: "todo" });

  const store = new DefaultGrapesStore({ cwd, grapesCommand: "" });
  const issue = await store.getIssue(1);
  assert.ok(issue);

  const result = await new DefaultPolicy().shouldDispatch(issue, await store.listIssues());
  assert.deepEqual(result, { ok: false, reason: "blocked by #2" });
});

class FakeWorktreeManager implements WorktreeManager {
  constructor(private readonly cwd: string) {}

  async ensureWorktree(issue: GrapesIssue): Promise<WorktreeInfo> {
    const path = join(this.cwd, ".worktrees", `issue-${issue.id}`);
    await mkdir(join(path, ".grapes", String(issue.id)), { recursive: true });
    return { issueId: issue.id, path, branch: `test/issue-${issue.id}`, created: true };
  }

  async recreateWorktree(issue: GrapesIssue): Promise<WorktreeInfo> {
    return this.ensureWorktree(issue);
  }
}

class ArtifactRunner implements AgentRunner {
  async runImplementation(context: DispatchContext): Promise<RunnerResult> {
    const artifactPath = join(context.worktree.path, ".grapes", String(context.issue.id), "implementation.md");
    await writeFile(
      artifactPath,
      [
        "## Summary",
        "",
        "Implemented the requested change.",
        "",
        "## Changed Files",
        "",
        "- src/example.ts",
        "",
        "## Tests Run",
        "",
        "- npm test",
        "",
        "## Risks",
        "",
        "- None",
        "",
        "Ready For Verification: yes",
      ].join("\n"),
      "utf8",
    );
    return {
      artifact: {
        summary: "Implemented the requested change.",
        changedFiles: ["src/example.ts"],
        testsRun: ["npm test"],
        risks: [],
        readyForVerification: true,
        artifactPath,
      },
      sessionFile: "/tmp/session.jsonl",
      sessionId: "abc",
    };
  }
}

class PassingVerifier implements Verifier {
  async verify(): Promise<VerificationDecision> {
    return {
      status: "done",
      summary: "Verification passed.",
      findings: [],
      testsRun: ["npm test"],
      risks: [],
      followUpIssues: [],
    };
  }
}

class NoopWatcher implements FileWatcher {
  async start(): Promise<void> {}
  async add(): Promise<void> {}
  async stop(): Promise<void> {}
}

async function writeIssue(
  cwd: string,
  options: {
    id?: number;
    status: "backlog" | "todo" | "in_progress" | "review" | "done" | "cancelled";
    labels?: string[];
    blockedBy?: number[];
  },
): Promise<void> {
  const id = options.id ?? 1;
  const dir = join(cwd, ".grapes", String(id));
  await mkdir(dir, { recursive: true });
  await writeFile(
    join(dir, "meta.toml"),
    [
      `title = 'Issue ${id}'`,
      `status = '${options.status}'`,
      "priority = 'high'",
      `labels = [${(options.labels ?? []).map((label) => `'${label}'`).join(", ")}]`,
      `blocked_by = [${(options.blockedBy ?? []).join(", ")}]`,
      "created = 2026-04-28T21:17:00Z",
      "updated = 2026-04-28T21:17:00Z",
    ].join("\n"),
    "utf8",
  );
  await writeFile(join(dir, "content.md"), "Do the task.\n", "utf8");
  await writeFile(join(dir, "comments.md"), "", "utf8");
}

function once<K extends "status_changed">(eventBus: ReturnType<typeof createEventBus>, event: K): Promise<unknown> {
  return new Promise((resolve) => {
    const off = eventBus.on(event, (payload) => {
      off();
      resolve(payload);
    });
  });
}
