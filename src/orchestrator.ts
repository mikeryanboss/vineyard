import { cpus, hostname } from "node:os";
import { join, resolve } from "node:path";
import { createEventBus } from "./event-bus.js";
import { DefaultGrapesStore } from "./grapes-store.js";
import { DefaultPolicy } from "./policy.js";
import { PiRpcRunner } from "./pi-runner.js";
import { ConservativeVerifier } from "./verifier.js";
import { ChokidarFileWatcher } from "./watcher.js";
import { GitWorktreeManager } from "./worktree-manager.js";
import { DefaultWorkflowLoader } from "./workflow-loader.js";
import type {
  AgentRunner,
  ClaimState,
  DispatchContext,
  EventBus,
  FileWatcher,
  GrapesIssue,
  GrapesStatus,
  GrapesStore,
  MachinePhase,
  OrchestratorOptions,
  ReviewPolicy,
  RunnerResult,
  Verifier,
  WorkflowLoader,
  WorktreeManager,
} from "./types.js";

export function createOrchestrator(options: OrchestratorOptions = {}): OrchestratorRuntime {
  const cwd = resolve(options.cwd ?? process.cwd());
  const eventBus = options.eventBus ?? createEventBus();
  return new OrchestratorRuntime({
    cwd,
    orchestratorId: options.orchestratorId ?? `${hostname()}-${process.pid}`,
    grapesStore: options.grapesStore ?? new DefaultGrapesStore({ cwd }),
    workflowLoader: options.workflowLoader ?? new DefaultWorkflowLoader({ cwd }),
    worktreeManager: options.worktreeManager ?? new GitWorktreeManager({ cwd }),
    runner: options.runner ?? new PiRpcRunner(),
    verifier: options.verifier ?? new ConservativeVerifier(),
    policy: options.policy ?? new DefaultPolicy(),
    watcher: options.watcher ?? new ChokidarFileWatcher(),
    eventBus,
    autoStartWatcher: options.autoStartWatcher ?? true,
  });
}

interface RuntimeServices {
  cwd: string;
  orchestratorId: string;
  grapesStore: GrapesStore;
  workflowLoader: WorkflowLoader;
  worktreeManager: WorktreeManager;
  runner: AgentRunner;
  verifier: Verifier;
  policy: ReviewPolicy;
  watcher: FileWatcher;
  eventBus: EventBus;
  autoStartWatcher: boolean;
}

interface RunningIssue {
  controller: AbortController;
  promise: Promise<void>;
}

export class OrchestratorRuntime {
  private readonly services: RuntimeServices;
  private readonly running = new Map<number, RunningIssue>();
  private started = false;
  private reconcileChain = Promise.resolve();

  constructor(services: RuntimeServices) {
    this.services = services;
  }

  get cwd(): string {
    return this.services.cwd;
  }

  get eventBus(): EventBus {
    return this.services.eventBus;
  }

  async start(): Promise<void> {
    if (this.started) return;
    this.started = true;
    this.eventBus.emit("started", { cwd: this.cwd });

    if (this.services.autoStartWatcher) {
      await this.services.watcher.start([join(this.cwd, ".grapes"), join(this.cwd, "WORKFLOW.md")], (path) => {
        this.enqueueReconcile(`file:${path}`);
      });
    }

    await this.reconcile("startup");
  }

  async stop(): Promise<void> {
    if (!this.started) return;
    this.started = false;
    await this.services.watcher.stop();

    for (const entry of this.running.values()) {
      entry.controller.abort();
    }
    await Promise.allSettled([...this.running.values()].map((entry) => entry.promise));
    this.running.clear();
    this.eventBus.emit("stopped", { cwd: this.cwd });
  }

  enqueueReconcile(reason: string): void {
    this.reconcileChain = this.reconcileChain
      .catch(() => undefined)
      .then(() => this.reconcile(reason));
  }

  async reconcile(reason = "manual"): Promise<void> {
    this.eventBus.emit("reconcile_start", { reason });

    try {
      const workflow = await this.services.workflowLoader.load();
      const issues = await this.services.grapesStore.listIssues();

      for (const issue of issues) {
        this.eventBus.emit("issue_discovered", { issue });
      }

      await this.stopIneligibleRunningIssues(issues);

      const sorted = [...issues].sort(compareIssues);
      for (const issue of sorted) {
        if (this.running.has(issue.id)) continue;
        const eligibility = await this.services.policy.shouldDispatch(issue, issues);
        if (!eligibility.ok) {
          this.eventBus.emit("issue_skipped", { issue, reason: eligibility.reason });
          continue;
        }
        await this.dispatch(issue, workflow);
      }
    } finally {
      this.eventBus.emit("reconcile_end", { reason });
    }
  }

  private async stopIneligibleRunningIssues(issues: readonly GrapesIssue[]): Promise<void> {
    const byId = new Map(issues.map((issue) => [issue.id, issue]));
    for (const [issueId, running] of this.running) {
      const issue = byId.get(issueId);
      if (!issue || issue.status === "done" || issue.status === "cancelled" || issue.status === "backlog") {
        running.controller.abort();
      }
    }
  }

  private async dispatch(issue: GrapesIssue, workflow: DispatchContext["workflow"]): Promise<void> {
    const controller = new AbortController();
    const promise = this.runIssue(issue, workflow, controller).finally(() => {
      this.running.delete(issue.id);
    });
    this.running.set(issue.id, { controller, promise });
  }

  private async runIssue(issue: GrapesIssue, workflow: DispatchContext["workflow"], controller: AbortController): Promise<void> {
    const attempt = await this.nextAttempt(issue.id);
    try {
      this.eventBus.emit("issue_claimed", { issue, attempt });
      await this.services.grapesStore.setStatus(issue.id, "in_progress");
      await this.writePhase(issue.id, "claimed", attempt);

      const worktree = await this.services.worktreeManager.ensureWorktree(issue);
      await this.services.grapesStore.writeWorktree(issue.id, worktree);
      await this.writePhase(issue.id, "worktree_ready", attempt, { worktree: worktree.path });

      await this.services.watcher.add([
        join(worktree.path, ".grapes", String(issue.id), "implementation.md"),
        join(worktree.path, ".grapes", String(issue.id), "question.md"),
        join(worktree.path, ".grapes", String(issue.id), "triage.md"),
        join(worktree.path, ".grapes", String(issue.id), "continuation.md"),
        join(worktree.path, ".grapes", String(issue.id), "verification.md"),
        join(worktree.path, ".grapes", String(issue.id), "review.md"),
      ]);

      const context: DispatchContext = { issue, workflow, worktree, attempt };

      await this.writePhase(issue.id, "implementing", attempt, { worktree: worktree.path });
      this.eventBus.emit("worker_started", { issue, worktree });
      const result = await this.services.runner.runImplementation(context, controller.signal);
      this.eventBus.emit("worker_finished", { issue, result });
      await this.recordRunnerResult(issue.id, attempt, result, worktree.path);

      await this.writePhase(issue.id, "awaiting_verification", attempt, { worktree: worktree.path });
      await this.writePhase(issue.id, "verifying", attempt, { worktree: worktree.path });
      const rawDecision = await this.services.verifier.verify(context, result.artifact, controller.signal);
      const decision = this.services.policy.applyReviewGate(issue, rawDecision);
      this.eventBus.emit("verification_finished", { issue, decision });

      await this.services.grapesStore.writeReviewPacket(issue.id, decision);
      await this.services.grapesStore.setStatus(issue.id, decision.status);
      this.eventBus.emit("status_changed", { issueId: issue.id, status: decision.status });
      await this.writePhase(issue.id, finalPhase(decision.status), attempt, { worktree: worktree.path });
    } catch (error) {
      const normalized = error instanceof Error ? error : new Error(String(error));
      this.eventBus.emit("error", { issueId: issue.id, error: normalized, phase: "failed" });
      await this.writePhase(issue.id, "failed", attempt).catch(() => undefined);
      await this.services.grapesStore.appendComment(issue.id, `[orchestrator] Failed: ${normalized.message}`).catch(
        () => undefined,
      );
      await this.services.grapesStore.setStatus(issue.id, "todo").catch(() => undefined);
    }
  }

  private async nextAttempt(issueId: number): Promise<number> {
    const claim = await this.services.grapesStore.readClaim(issueId);
    return (claim?.attempt ?? 0) + 1;
  }

  private async writePhase(
    issueId: number,
    phase: MachinePhase,
    attempt: number,
    extra: Partial<ClaimState> = {},
  ): Promise<void> {
    const claim: ClaimState = {
      issueId,
      claimedBy: this.services.orchestratorId,
      phase,
      attempt,
      updatedAt: new Date().toISOString(),
      ...extra,
    };
    await this.services.grapesStore.writeClaim(issueId, claim);
    this.eventBus.emit("phase_changed", { issueId, phase });
  }

  private async recordRunnerResult(
    issueId: number,
    attempt: number,
    result: RunnerResult,
    worktree: string,
  ): Promise<void> {
    await this.services.grapesStore.writeRun(issueId, {
      attempt,
      role: "implementation",
      runner: "pi",
      phase: "awaiting_verification",
      worktree,
      implementation_session_file: result.sessionFile,
      implementation_session_id: result.sessionId,
      last_event_at: new Date().toISOString(),
    });
  }
}

function compareIssues(a: GrapesIssue, b: GrapesIssue): number {
  const byPriority = priorityRank(a.priority) - priorityRank(b.priority);
  if (byPriority !== 0) return byPriority;
  return a.id - b.id;
}

function priorityRank(priority: GrapesIssue["priority"]): number {
  switch (priority) {
    case "urgent":
      return 0;
    case "high":
      return 1;
    case "medium":
      return 2;
    case "low":
      return 3;
  }
}

function finalPhase(status: GrapesStatus): MachinePhase {
  switch (status) {
    case "done":
    case "review":
    case "cancelled":
      return "complete";
    case "todo":
      return "awaiting_rework";
    case "backlog":
      return "idle";
    case "in_progress":
      return "verifying";
  }
}

export function defaultConcurrency(): number {
  return Math.max(1, Math.floor(cpus().length / 2));
}
