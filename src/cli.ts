#!/usr/bin/env node
import { createOrchestrator } from "./orchestrator.js";

async function main(argv: string[]): Promise<void> {
  const command = argv[2] ?? "run";
  const cwd = readOption(argv, "--cwd") ?? process.cwd();
  const once = argv.includes("--once") || command === "reconcile";

  const orchestrator = createOrchestrator({
    cwd,
    autoStartWatcher: !once,
  });

  orchestrator.eventBus.on("error", ({ issueId, error }) => {
    const prefix = issueId === undefined ? "orchestrator" : `issue #${issueId}`;
    console.error(`[${prefix}] ${error.message}`);
  });

  if (once) {
    await orchestrator.reconcile("cli");
    return;
  }

  await orchestrator.start();
  const stop = async () => {
    await orchestrator.stop();
    process.exit(0);
  };
  process.once("SIGINT", () => void stop());
  process.once("SIGTERM", () => void stop());
}

function readOption(argv: string[], name: string): string | undefined {
  const index = argv.indexOf(name);
  return index === -1 ? undefined : argv[index + 1];
}

main(process.argv).catch((error: unknown) => {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
});
