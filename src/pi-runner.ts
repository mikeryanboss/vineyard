import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { mkdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import { createInterface } from "node:readline";
import type { AgentRunner, DispatchContext, RunnerResult, WorkerArtifact } from "./types.js";

export interface PiRpcRunnerOptions {
  command?: string;
  args?: string[];
  provider?: string;
  model?: string;
}

export class PiRpcRunner implements AgentRunner {
  private readonly command: string;
  private readonly args: string[];
  private readonly provider: string | undefined;
  private readonly model: string | undefined;

  constructor(options: PiRpcRunnerOptions = {}) {
    this.command = options.command ?? "pi";
    this.args = options.args ?? ["--mode", "rpc"];
    this.provider = options.provider;
    this.model = options.model;
  }

  async runImplementation(context: DispatchContext, signal: AbortSignal): Promise<RunnerResult> {
    const sessionDir = join(context.worktree.path, ".grapes", String(context.issue.id), "pi-session");
    await mkdir(sessionDir, { recursive: true });

    const args = [...this.args, "--session-dir", sessionDir];
    if (this.provider) args.push("--provider", this.provider);
    if (this.model) args.push("--model", this.model);

    const child = spawn(this.command, args, {
      cwd: context.worktree.path,
      stdio: ["pipe", "pipe", "pipe"],
    });

    const cleanup = () => {
      if (!child.killed) child.kill("SIGTERM");
    };
    signal.addEventListener("abort", cleanup, { once: true });

    try {
      const state = await runPrompt(child, buildImplementationPrompt(context));
      const artifact = await readImplementationArtifact(context);
      return stripUndefined({
        artifact,
        sessionFile: state.sessionFile,
        sessionId: state.sessionId,
      }) as RunnerResult;
    } finally {
      signal.removeEventListener("abort", cleanup);
      cleanup();
    }
  }
}

async function runPrompt(
  child: ChildProcessWithoutNullStreams,
  message: string,
): Promise<{ sessionFile?: string; sessionId?: string }> {
  const stdout = createInterface({ input: child.stdout });
  const stderr: string[] = [];
  child.stderr.on("data", (chunk) => stderr.push(String(chunk)));

  let sessionFile: string | undefined;
  let sessionId: string | undefined;
  let agentEnded = false;
  let requestedState = false;

  const send = (command: Record<string, unknown>) => {
    child.stdin.write(`${JSON.stringify(command)}\n`);
  };

  const done = new Promise<void>((resolve, reject) => {
    child.once("error", reject);
    child.once("exit", (code) => {
      if (agentEnded || code === 0) resolve();
      else reject(new Error(`pi exited with code ${code}: ${stderr.join("").trim()}`));
    });

    stdout.on("line", (line) => {
      if (!line.trim()) return;
      const event = parseJsonLine(line);
      if (!event) return;
      if (event.type === "response" && event.command === "get_state" && event.success === true) {
        const data = event.data as { sessionFile?: string; sessionId?: string } | undefined;
        sessionFile = data?.sessionFile;
        sessionId = data?.sessionId;
        if (agentEnded && requestedState) resolve();
      }
      if (event.type === "agent_end") {
        agentEnded = true;
        requestedState = true;
        send({ id: "state-1", type: "get_state" });
      }
    });
  });

  send({ id: "prompt-1", type: "prompt", message });
  await done;
  return stripUndefined({ sessionFile, sessionId }) as { sessionFile?: string; sessionId?: string };
}

async function readImplementationArtifact(context: DispatchContext): Promise<WorkerArtifact | undefined> {
  const path = join(context.worktree.path, ".grapes", String(context.issue.id), "implementation.md");
  const content = await readFile(path, "utf8").catch((error: unknown) => {
    if (isNodeError(error) && error.code === "ENOENT") return undefined;
    throw error;
  });
  if (!content) return undefined;

  return {
    summary: extractSection(content, "Summary") || content.trim(),
    changedFiles: extractList(content, "Changed Files"),
    testsRun: extractList(content, "Tests Run"),
    risks: extractList(content, "Risks"),
    readyForVerification: /ready\s*for\s*verification\s*:\s*yes/i.test(content),
    artifactPath: path,
  };
}

function buildImplementationPrompt(context: DispatchContext): string {
  return [
    `You are working on Grapes issue ${context.issue.identifier}.`,
    "",
    "## Issue Metadata",
    "",
    `- Title: ${context.issue.title}`,
    `- Status: ${context.issue.status}`,
    `- Priority: ${context.issue.priority}`,
    `- Labels: ${context.issue.labels.join(", ") || "none"}`,
    `- Worktree: ${context.worktree.path}`,
    "",
    "## Task",
    "",
    context.issue.body,
    "",
    "## Prior Context",
    "",
    context.issue.comments,
    "",
    "## Workflow",
    "",
    context.workflow.prompt,
    "",
    "## Completion Artifact",
    "",
    `Write .grapes/${context.issue.id}/implementation.md with Summary, Changed Files, Tests Run, Risks, and Ready For Verification: yes/no.`,
    "Do not mark the issue done. The orchestrator/verifier owns status transitions.",
  ].join("\n");
}

function extractSection(content: string, heading: string): string {
  const pattern = new RegExp(`^##\\s+${escapeRegExp(heading)}\\s*$([\\s\\S]*?)(?=^##\\s+|(?![\\s\\S]))`, "im");
  const match = content.match(pattern);
  return match?.[1]?.trim() ?? "";
}

function extractList(content: string, heading: string): string[] {
  return extractSection(content, heading)
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.startsWith("- "))
    .map((line) => line.slice(2).trim())
    .filter(Boolean);
}

function parseJsonLine(line: string): Record<string, unknown> | undefined {
  try {
    const value = JSON.parse(line) as unknown;
    return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : undefined;
  } catch {
    return undefined;
  }
}

function escapeRegExp(input: string): string {
  return input.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}

function stripUndefined<T extends Record<string, unknown>>(value: T): Partial<T> {
  return Object.fromEntries(Object.entries(value).filter(([, entry]) => entry !== undefined)) as Partial<T>;
}
