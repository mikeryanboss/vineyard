import { access, appendFile, mkdir, readFile, readdir, rename, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { join, resolve } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { parseFlatToml, stringifyFlatToml } from "./toml.js";
import type {
  ClaimState,
  GrapesIssue,
  GrapesPriority,
  GrapesStatus,
  GrapesStore,
  VerificationDecision,
  WorktreeInfo,
} from "./types.js";

const execFileAsync = promisify(execFile);

const validStatuses = new Set<GrapesStatus>(["backlog", "todo", "in_progress", "review", "done", "cancelled"]);
const validPriorities = new Set<GrapesPriority>(["urgent", "high", "medium", "low"]);

export interface DefaultGrapesStoreOptions {
  cwd: string;
  grapesDir?: string;
  grapesCommand?: string;
}

export class DefaultGrapesStore implements GrapesStore {
  private readonly cwd: string;
  private readonly grapesDir: string;
  private readonly grapesCommand: string | undefined;

  constructor(options: DefaultGrapesStoreOptions) {
    this.cwd = resolve(options.cwd);
    this.grapesDir = resolve(this.cwd, options.grapesDir ?? ".grapes");
    this.grapesCommand = options.grapesCommand;
  }

  async listIssues(): Promise<GrapesIssue[]> {
    const entries = await readdir(this.grapesDir, { withFileTypes: true }).catch((error: unknown) => {
      if (isNodeError(error) && error.code === "ENOENT") return [];
      throw error;
    });

    const ids = entries
      .filter((entry) => entry.isDirectory() && /^\d+$/.test(entry.name))
      .map((entry) => Number(entry.name))
      .sort((a, b) => a - b);

    const issues: GrapesIssue[] = [];
    for (const id of ids) {
      const issue = await this.getIssue(id);
      if (issue) issues.push(issue);
    }
    return issues;
  }

  async getIssue(id: number): Promise<GrapesIssue | undefined> {
    const directory = this.issueDir(id);
    const metaPath = join(directory, "meta.toml");

    let metaContent: string;
    try {
      metaContent = await readFile(metaPath, "utf8");
    } catch (error) {
      if (isNodeError(error) && error.code === "ENOENT") return undefined;
      throw error;
    }

    const meta = parseFlatToml(metaContent);
    const title = asString(meta.title, `Issue #${id}`);
    const status = parseStatus(meta.status);
    const priority = parsePriority(meta.priority);
    const labels = asStringArray(meta.labels);
    const blockedBy = asNumberArray(meta.blocked_by);
    const parent = asOptionalNumber(meta.parent);
    const body = await readOptionalFile(join(directory, "content.md"));
    const comments = await readOptionalFile(join(directory, "comments.md"));

    return stripUndefined({
      id,
      identifier: `#${id}`,
      directory,
      title,
      status,
      priority,
      labels,
      ...(parent === undefined ? {} : { parent }),
      blockedBy,
      created: asOptionalString(meta.created),
      updated: asOptionalString(meta.updated),
      body,
      comments,
    }) as GrapesIssue;
  }

  async setStatus(issueId: number, status: GrapesStatus): Promise<void> {
    const metaPath = join(this.issueDir(issueId), "meta.toml");
    const content = await readFile(metaPath, "utf8");
    const next = replaceTomlField(content, "status", `'${status}'`);
    await writeAtomic(metaPath, next);
    await this.touchIssue(issueId);
  }

  async appendComment(issueId: number, body: string): Promise<void> {
    const commentsPath = join(this.issueDir(issueId), "comments.md");
    const timestamp = new Date().toISOString().slice(0, 16);
    const normalizedBody = body.endsWith("\n") ? body : `${body}\n`;
    await appendFile(commentsPath, `\n### ${timestamp}\n${normalizedBody}`, "utf8");
    await this.touchIssue(issueId);
  }

  async readClaim(issueId: number): Promise<ClaimState | undefined> {
    const path = join(this.issueDir(issueId), "claim.toml");
    const content = await readOptionalFile(path);
    if (!content) return undefined;
    const data = parseFlatToml(content);
    return stripUndefined({
      issueId,
      claimedBy: asString(data.claimed_by, ""),
      phase: parsePhase(data.phase),
      attempt: Number(data.attempt ?? 0),
      worktree: asOptionalString(data.worktree),
      implementationSessionFile: asOptionalString(data.implementation_session_file),
      implementationSessionId: asOptionalString(data.implementation_session_id),
      verifierSessionFile: asOptionalString(data.verifier_session_file),
      verifierSessionId: asOptionalString(data.verifier_session_id),
      updatedAt: asString(data.updated_at, ""),
    }) as ClaimState;
  }

  async writeClaim(issueId: number, claim: ClaimState): Promise<void> {
    await writeAtomic(
      join(this.issueDir(issueId), "claim.toml"),
      stringifyFlatToml({
        issue_id: issueId,
        claimed_by: claim.claimedBy,
        phase: claim.phase,
        attempt: claim.attempt,
        worktree: claim.worktree,
        implementation_session_file: claim.implementationSessionFile,
        implementation_session_id: claim.implementationSessionId,
        verifier_session_file: claim.verifierSessionFile,
        verifier_session_id: claim.verifierSessionId,
        updated_at: claim.updatedAt,
      }),
    );
    await this.touchIssue(issueId);
  }

  async writeRun(issueId: number, data: Record<string, unknown>): Promise<void> {
    await writeAtomic(join(this.issueDir(issueId), "run.toml"), stringifyFlatToml(data));
    await this.touchIssue(issueId);
  }

  async writeWorktree(issueId: number, worktree: WorktreeInfo): Promise<void> {
    await writeAtomic(
      join(this.issueDir(issueId), "worktree.toml"),
      stringifyFlatToml({
        issue_id: issueId,
        path: worktree.path,
        branch: worktree.branch,
        created: worktree.created,
      }),
    );
    await this.touchIssue(issueId);
  }

  async writeReviewPacket(issueId: number, decision: VerificationDecision): Promise<void> {
    const packet = [
      "## Verification Result",
      "",
      `Status: ${decision.status}`,
      "",
      "## Summary",
      "",
      decision.summary || "No summary provided.",
      "",
      "## Findings",
      "",
      ...listOrNone(decision.findings),
      "",
      "## Tests Run",
      "",
      ...listOrNone(decision.testsRun),
      "",
      "## Risks",
      "",
      ...listOrNone(decision.risks),
      "",
      "## Follow-Up Issues",
      "",
      ...listOrNone(decision.followUpIssues.map((id) => `#${id}`)),
      "",
    ].join("\n");
    await writeAtomic(join(this.issueDir(issueId), "review.md"), packet);
    await this.touchIssue(issueId);
  }

  private issueDir(id: number): string {
    return join(this.grapesDir, String(id));
  }

  private async touchIssue(issueId: number): Promise<void> {
    if (this.grapesCommand === "") return;
    const command = this.grapesCommand ?? "grapes";
    try {
      await execFileAsync(command, ["issue", String(issueId)], { cwd: this.cwd });
    } catch {
      // The store remains usable in tests and embedded deployments without the Grapes CLI.
    }
  }
}

async function readOptionalFile(path: string): Promise<string> {
  try {
    return await readFile(path, "utf8");
  } catch (error) {
    if (isNodeError(error) && error.code === "ENOENT") return "";
    throw error;
  }
}

async function writeAtomic(path: string, content: string): Promise<void> {
  await mkdir(resolve(path, ".."), { recursive: true });
  const tmp = `${path}.${process.pid}.${Date.now()}.tmp`;
  await writeFile(tmp, content, "utf8");
  await rename(tmp, path);
}

async function pathExists(path: string): Promise<boolean> {
  try {
    await access(path, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

export async function ensureDirectory(path: string): Promise<void> {
  if (!(await pathExists(path))) {
    await mkdir(path, { recursive: true });
  }
}

function replaceTomlField(content: string, key: string, value: string): string {
  const line = `${key} = ${value}`;
  const pattern = new RegExp(`^${key}\\s*=.*$`, "m");
  if (pattern.test(content)) return content.replace(pattern, line);
  return content.endsWith("\n") ? `${content}${line}\n` : `${content}\n${line}\n`;
}

function asString(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}

function asOptionalString(value: unknown): string | undefined {
  return typeof value === "string" ? value : undefined;
}

function asOptionalNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string") : [];
}

function asNumberArray(value: unknown): number[] {
  return Array.isArray(value) ? value.filter((item): item is number => typeof item === "number") : [];
}

function parseStatus(value: unknown): GrapesStatus {
  return typeof value === "string" && validStatuses.has(value as GrapesStatus) ? (value as GrapesStatus) : "backlog";
}

function parsePriority(value: unknown): GrapesPriority {
  return typeof value === "string" && validPriorities.has(value as GrapesPriority)
    ? (value as GrapesPriority)
    : "medium";
}

function parsePhase(value: unknown): ClaimState["phase"] {
  const phase = asString(value, "idle");
  switch (phase) {
    case "idle":
    case "claimed":
    case "worktree_ready":
    case "implementing":
    case "awaiting_verification":
    case "verifying":
    case "awaiting_rework":
    case "complete":
    case "failed":
      return phase;
    default:
      return "idle";
  }
}

function listOrNone(items: readonly string[]): string[] {
  return items.length > 0 ? items.map((item) => `- ${item}`) : ["- None"];
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}

function stripUndefined<T extends Record<string, unknown>>(value: T): Partial<T> {
  return Object.fromEntries(Object.entries(value).filter(([, entry]) => entry !== undefined)) as Partial<T>;
}
