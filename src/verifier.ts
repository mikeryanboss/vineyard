import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { DispatchContext, VerificationDecision, Verifier, WorkerArtifact } from "./types.js";

export class ConservativeVerifier implements Verifier {
  async verify(context: DispatchContext, artifact: WorkerArtifact | undefined): Promise<VerificationDecision> {
    const explicitDecision = await readVerificationArtifact(context);
    if (explicitDecision) return explicitDecision;

    return {
      status: "review",
      summary: artifact?.summary ?? "Implementation finished, but no verifier decision artifact was available.",
      findings: ["No explicit verification artifact was found; human review is required."],
      testsRun: artifact?.testsRun ?? [],
      risks: artifact?.risks ?? ["Completion was not independently verified."],
      followUpIssues: [],
    };
  }
}

async function readVerificationArtifact(context: DispatchContext): Promise<VerificationDecision | undefined> {
  const path = join(context.worktree.path, ".grapes", String(context.issue.id), "verification.md");
  const content = await readFile(path, "utf8").catch((error: unknown) => {
    if (isNodeError(error) && error.code === "ENOENT") return undefined;
    throw error;
  });
  if (!content) return undefined;

  const status = parseStatus(content);
  if (!status) return undefined;

  return {
    status,
    summary: extractSection(content, "Summary") || "Verifier decision artifact was provided.",
    findings: extractList(content, "Findings"),
    testsRun: extractList(content, "Tests Run"),
    risks: extractList(content, "Risks"),
    followUpIssues: extractList(content, "Follow-Up Issues")
      .map((item) => item.match(/#?(\d+)/)?.[1])
      .filter((item): item is string => Boolean(item))
      .map(Number),
  };
}

function parseStatus(content: string): VerificationDecision["status"] | undefined {
  const match = content.match(/^Status:\s*(done|review|todo|backlog|cancelled)\s*$/im);
  return match?.[1] as VerificationDecision["status"] | undefined;
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
    .filter((line) => line && line.toLowerCase() !== "none");
}

function escapeRegExp(input: string): string {
  return input.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}
