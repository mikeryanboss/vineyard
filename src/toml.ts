type TomlScalar = string | number | boolean | string[] | number[];

export function parseFlatToml(input: string): Record<string, TomlScalar> {
  const result: Record<string, TomlScalar> = {};

  for (const rawLine of input.split(/\r?\n/)) {
    const line = stripComment(rawLine).trim();
    if (!line || line.startsWith("[")) continue;

    const separator = line.indexOf("=");
    if (separator === -1) continue;

    const key = line.slice(0, separator).trim();
    const value = line.slice(separator + 1).trim();
    if (!key) continue;
    result[key] = parseValue(value);
  }

  return result;
}

export function stringifyFlatToml(values: Record<string, unknown>): string {
  const lines: string[] = [];
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined) continue;
    lines.push(`${key} = ${formatValue(value)}`);
  }
  return `${lines.join("\n")}\n`;
}

function stripComment(line: string): string {
  let quote: "'" | '"' | undefined;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    if ((char === "'" || char === '"') && line[i - 1] !== "\\") {
      quote = quote === char ? undefined : quote ?? char;
    }
    if (char === "#" && !quote) return line.slice(0, i);
  }
  return line;
}

function parseValue(value: string): TomlScalar {
  if (value.startsWith("[") && value.endsWith("]")) {
    const inner = value.slice(1, -1).trim();
    if (!inner) return [];
    return splitArray(inner).map((part) => parseValue(part)) as string[] | number[];
  }
  if ((value.startsWith("'") && value.endsWith("'")) || (value.startsWith('"') && value.endsWith('"'))) {
    return value.slice(1, -1).replace(/\\"/g, '"').replace(/\\'/g, "'");
  }
  if (value === "true") return true;
  if (value === "false") return false;
  if (/^-?\d+(\.\d+)?$/.test(value)) return Number(value);
  return value;
}

function splitArray(input: string): string[] {
  const parts: string[] = [];
  let current = "";
  let quote: "'" | '"' | undefined;

  for (let i = 0; i < input.length; i += 1) {
    const char = input[i];
    if ((char === "'" || char === '"') && input[i - 1] !== "\\") {
      quote = quote === char ? undefined : quote ?? char;
    }
    if (char === "," && !quote) {
      parts.push(current.trim());
      current = "";
    } else {
      current += char;
    }
  }

  if (current.trim()) parts.push(current.trim());
  return parts;
}

function formatValue(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(formatValue).join(", ")}]`;
  if (typeof value === "string") {
    if (/^\d{4}-\d{2}-\d{2}T/.test(value)) return value;
    return `'${value.replace(/'/g, "\\'")}'`;
  }
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  if (value === null) return "''";
  return `'${String(value).replace(/'/g, "\\'")}'`;
}
