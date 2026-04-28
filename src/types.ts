export type GrapesStatus = "backlog" | "todo" | "in_progress" | "review" | "done" | "cancelled";

export type GrapesPriority = "urgent" | "high" | "medium" | "low";

export type MachinePhase =
  | "idle"
  | "claimed"
  | "worktree_ready"
  | "implementing"
  | "awaiting_verification"
  | "verifying"
  | "awaiting_rework"
  | "complete"
  | "failed";

export type PhaseRole = "implementer" | "verifier" | "planner" | "landing";

export interface GrapesIssue {
  id: number;
  identifier: `#${number}`;
  directory: string;
  title: string;
  status: GrapesStatus;
  priority: GrapesPriority;
  labels: string[];
  parent?: number;
  blockedBy: number[];
  created?: string;
  updated?: string;
  body: string;
  comments: string;
}

export interface WorkflowDefinition {
  path: string;
  prompt: string;
  config: Record<string, unknown>;
}

export interface WorktreeInfo {
  issueId: number;
  path: string;
  created: boolean;
  branch: string;
}

export interface ClaimState {
  issueId: number;
  claimedBy: string;
  phase: MachinePhase;
  attempt: number;
  worktree?: string;
  implementationSessionFile?: string;
  implementationSessionId?: string;
  verifierSessionFile?: string;
  verifierSessionId?: string;
  updatedAt: string;
}

export interface WorkerArtifact {
  summary: string;
  changedFiles: string[];
  testsRun: string[];
  risks: string[];
  readyForVerification: boolean;
  artifactPath?: string;
}

export type VerificationStatus = "done" | "review" | "todo" | "backlog" | "cancelled";

export interface VerificationDecision {
  status: VerificationStatus;
  summary: string;
  findings: string[];
  testsRun: string[];
  risks: string[];
  followUpIssues: number[];
}

export interface DispatchContext {
  issue: GrapesIssue;
  workflow: WorkflowDefinition;
  worktree: WorktreeInfo;
  attempt: number;
}

export interface RunnerResult {
  artifact?: WorkerArtifact;
  sessionFile?: string;
  sessionId?: string;
}

export interface EventMap {
  started: { cwd: string };
  stopped: { cwd: string };
  reconcile_start: { reason: string };
  reconcile_end: { reason: string };
  issue_discovered: { issue: GrapesIssue };
  issue_skipped: { issue: GrapesIssue; reason: string };
  issue_claimed: { issue: GrapesIssue; attempt: number };
  phase_changed: { issueId: number; phase: MachinePhase };
  worker_started: { issue: GrapesIssue; worktree: WorktreeInfo };
  worker_finished: { issue: GrapesIssue; result: RunnerResult };
  verification_finished: { issue: GrapesIssue; decision: VerificationDecision };
  status_changed: { issueId: number; status: GrapesStatus };
  error: { issueId?: number; error: Error; phase?: MachinePhase };
}

export type OrchestratorEventName = keyof EventMap;

export interface EventBus {
  emit<K extends OrchestratorEventName>(event: K, data: EventMap[K]): void;
  on<K extends OrchestratorEventName>(event: K, handler: (data: EventMap[K]) => void | Promise<void>): () => void;
  clear(): void;
}

export interface GrapesStore {
  listIssues(): Promise<GrapesIssue[]>;
  getIssue(id: number): Promise<GrapesIssue | undefined>;
  setStatus(issueId: number, status: GrapesStatus): Promise<void>;
  appendComment(issueId: number, body: string): Promise<void>;
  readClaim(issueId: number): Promise<ClaimState | undefined>;
  writeClaim(issueId: number, claim: ClaimState): Promise<void>;
  writeRun(issueId: number, data: Record<string, unknown>): Promise<void>;
  writeWorktree(issueId: number, worktree: WorktreeInfo): Promise<void>;
  writeReviewPacket(issueId: number, decision: VerificationDecision): Promise<void>;
}

export interface WorkflowLoader {
  load(): Promise<WorkflowDefinition>;
}

export interface WorktreeManager {
  ensureWorktree(issue: GrapesIssue): Promise<WorktreeInfo>;
  recreateWorktree(issue: GrapesIssue): Promise<WorktreeInfo>;
}

export interface AgentRunner {
  runImplementation(context: DispatchContext, signal: AbortSignal): Promise<RunnerResult>;
}

export interface Verifier {
  verify(context: DispatchContext, artifact: WorkerArtifact | undefined, signal: AbortSignal): Promise<VerificationDecision>;
}

export interface ReviewPolicy {
  shouldDispatch(issue: GrapesIssue, allIssues: readonly GrapesIssue[]): Promise<{ ok: true } | { ok: false; reason: string }>;
  applyReviewGate(issue: GrapesIssue, decision: VerificationDecision): VerificationDecision;
}

export interface FileWatcher {
  start(paths: string[], onChange: (path: string) => void): Promise<void>;
  add(paths: string[]): Promise<void>;
  stop(): Promise<void>;
}

export interface OrchestratorOptions {
  cwd?: string;
  orchestratorId?: string;
  grapesStore?: GrapesStore;
  workflowLoader?: WorkflowLoader;
  worktreeManager?: WorktreeManager;
  runner?: AgentRunner;
  verifier?: Verifier;
  policy?: ReviewPolicy;
  watcher?: FileWatcher;
  eventBus?: EventBus;
  autoStartWatcher?: boolean;
}
