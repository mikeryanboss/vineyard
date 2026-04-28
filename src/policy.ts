import type { GrapesIssue, ReviewPolicy, VerificationDecision } from "./types.js";

const terminalStatuses = new Set(["done", "cancelled"]);
const defaultReviewLabels = new Set([
  "requires-human-review",
  "security",
  "auth",
  "billing",
  "migration",
  "public-api",
  "data-loss-risk",
  "production-config",
]);

export interface DefaultPolicyOptions {
  reviewLabels?: Iterable<string>;
}

export class DefaultPolicy implements ReviewPolicy {
  private readonly reviewLabels: Set<string>;

  constructor(options: DefaultPolicyOptions = {}) {
    this.reviewLabels = new Set(options.reviewLabels ?? defaultReviewLabels);
  }

  async shouldDispatch(
    issue: GrapesIssue,
    allIssues: readonly GrapesIssue[],
  ): Promise<{ ok: true } | { ok: false; reason: string }> {
    if (issue.status !== "todo") {
      return { ok: false, reason: `status is ${issue.status}` };
    }

    const byId = new Map(allIssues.map((candidate) => [candidate.id, candidate]));
    for (const blockerId of issue.blockedBy) {
      const blocker = byId.get(blockerId);
      if (!blocker || !terminalStatuses.has(blocker.status)) {
        return { ok: false, reason: `blocked by #${blockerId}` };
      }
    }

    return { ok: true };
  }

  applyReviewGate(issue: GrapesIssue, decision: VerificationDecision): VerificationDecision {
    if (decision.status !== "done") return decision;

    const requiresReview = issue.labels.some((label) => this.reviewLabels.has(label));
    if (!requiresReview) return decision;

    return {
      ...decision,
      status: "review",
      findings: [
        ...decision.findings,
        "Human review is required by issue label policy even though verification passed.",
      ],
    };
  }
}
