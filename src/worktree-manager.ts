import { access, mkdir, rm } from "node:fs/promises";
import { constants } from "node:fs";
import { join, resolve } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import type { GrapesIssue, WorktreeInfo, WorktreeManager } from "./types.js";

const execFileAsync = promisify(execFile);

export interface GitWorktreeManagerOptions {
  cwd: string;
  root?: string;
  baseRef?: string;
  gitCommand?: string;
}

export class GitWorktreeManager implements WorktreeManager {
  private readonly cwd: string;
  private readonly root: string;
  private readonly baseRef: string;
  private readonly gitCommand: string;

  constructor(options: GitWorktreeManagerOptions) {
    this.cwd = resolve(options.cwd);
    this.root = resolve(this.cwd, options.root ?? ".worktrees");
    this.baseRef = options.baseRef ?? "HEAD";
    this.gitCommand = options.gitCommand ?? "git";
  }

  async ensureWorktree(issue: GrapesIssue): Promise<WorktreeInfo> {
    const path = this.worktreePath(issue);
    const branch = this.branchName(issue);
    if (await exists(path)) {
      return { issueId: issue.id, path, branch, created: false };
    }

    await mkdir(this.root, { recursive: true });
    await execFileAsync(this.gitCommand, ["worktree", "add", "-b", branch, path, this.baseRef], { cwd: this.cwd });
    return { issueId: issue.id, path, branch, created: true };
  }

  async recreateWorktree(issue: GrapesIssue): Promise<WorktreeInfo> {
    const path = this.worktreePath(issue);
    if (await exists(path)) {
      await execFileAsync(this.gitCommand, ["worktree", "remove", "--force", path], { cwd: this.cwd }).catch(async () => {
        await rm(path, { recursive: true, force: true });
      });
    }
    return this.ensureWorktree(issue);
  }

  private worktreePath(issue: GrapesIssue): string {
    return join(this.root, `issue-${issue.id}`);
  }

  private branchName(issue: GrapesIssue): string {
    return `vineyard/issue-${issue.id}`;
  }
}

async function exists(path: string): Promise<boolean> {
  try {
    await access(path, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}
