import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import type { WorkflowDefinition, WorkflowLoader } from "./types.js";

export interface DefaultWorkflowLoaderOptions {
  cwd: string;
  path?: string;
}

export class DefaultWorkflowLoader implements WorkflowLoader {
  private readonly path: string;

  constructor(options: DefaultWorkflowLoaderOptions) {
    this.path = resolve(options.cwd, options.path ?? "WORKFLOW.md");
  }

  async load(): Promise<WorkflowDefinition> {
    const raw = await readFile(this.path, "utf8").catch((error: unknown) => {
      if (isNodeError(error) && error.code === "ENOENT") {
        return "";
      }
      throw error;
    });

    const { config, prompt } = parseFrontMatter(raw);
    return {
      path: this.path,
      prompt,
      config,
    };
  }
}

function parseFrontMatter(raw: string): { config: Record<string, unknown>; prompt: string } {
  if (!raw.startsWith("---\n")) {
    return { config: {}, prompt: raw.trim() };
  }

  const end = raw.indexOf("\n---", 4);
  if (end === -1) return { config: {}, prompt: raw.trim() };

  const configText = raw.slice(4, end).trim();
  const prompt = raw.slice(end + 4).trim();
  return { config: parseSimpleYamlObject(configText), prompt };
}

function parseSimpleYamlObject(input: string): Record<string, unknown> {
  const config: Record<string, unknown> = {};
  for (const rawLine of input.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const separator = line.indexOf(":");
    if (separator === -1) continue;
    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim();
    if (!key) continue;
    config[key] = parseScalar(value);
  }
  return config;
}

function parseScalar(value: string): unknown {
  if (value === "true") return true;
  if (value === "false") return false;
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
  if ((value.startsWith("'") && value.endsWith("'")) || (value.startsWith('"') && value.endsWith('"'))) {
    return value.slice(1, -1);
  }
  return value;
}

function isNodeError(error: unknown): error is NodeJS.ErrnoException {
  return error instanceof Error && "code" in error;
}
