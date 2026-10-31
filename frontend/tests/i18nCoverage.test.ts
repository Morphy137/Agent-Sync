import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { getDictionary } from "../src/lib/i18n";

const SRC_DIR = resolve(process.cwd(), "src");
const LOCALES_DIR = join(SRC_DIR, "lib", "i18n", "locales");
const en = getDictionary("en");
const SOURCE_FILE_PATTERN = /\.(svelte|ts)$/;
const STATIC_KEY_PATTERN = /\bt\(\s*["']([a-z0-9_.]+)["']/g;
const MARKUP_TEXT_PATTERN = />([^<>{}]*[A-Za-z]{2,}[^<>{}]*)</g;
const MARKUP_ATTR_PATTERN =
  /\b(label|placeholder|title|aria-label|ariaLabel|text|alt)="([^"{]*[A-Za-z][^"{]*)"/g;

function listSourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) return listSourceFiles(full);
    return SOURCE_FILE_PATTERN.test(entry.name) ? [full] : [];
  });
}

function listLocaleFiles(locale: string): string[] {
  const nsDir = join(LOCALES_DIR, locale);
  const nsFiles = readdirSync(LOCALES_DIR, { withFileTypes: true }).some(
    (e) => e.isDirectory() && e.name === locale
  )
    ? readdirSync(nsDir)
        .filter((f) => f.endsWith(".json"))
        .map((f) => join(nsDir, f))
    : [];
  return [join(LOCALES_DIR, `${locale}.json`), ...nsFiles];
}

function leafKeys(obj: Record<string, any>, prefix = ""): string[] {
  return Object.entries(obj).flatMap(([k, v]) => {
    const full = prefix ? `${prefix}.${k}` : k;
    return v && typeof v === "object" ? leafKeys(v, full) : [full];
  });
}

function lookup(obj: Record<string, any>, key: string): unknown {
  return key.split(".").reduce<any>((acc, part) => (acc ? acc[part] : undefined), obj);
}

/** Returns object keys declared more than once in the same JSON object (JSON.parse silently keeps the last). */
function findDuplicateJsonKeys(raw: string): string[] {
  const duplicates: string[] = [];
  const stack: { keys: Set<string>; path: string; lastKey: string }[] = [];
  let i = 0;
  while (i < raw.length) {
    const ch = raw[i];
    if (ch === '"') {
      let end = i + 1;
      while (raw[end] !== '"') end += raw[end] === "\\" ? 2 : 1;
      const value = JSON.parse(raw.slice(i, end + 1)) as string;
      let next = end + 1;
      while (/\s/.test(raw[next] ?? "")) next++;
      const frame = stack[stack.length - 1];
      if (raw[next] === ":" && frame) {
        const fullKey = frame.path ? `${frame.path}.${value}` : value;
        if (frame.keys.has(value)) duplicates.push(fullKey);
        frame.keys.add(value);
        frame.lastKey = value;
      }
      i = end + 1;
      continue;
    }
    if (ch === "{") {
      const parent = stack[stack.length - 1];
      const path = parent
        ? parent.path
          ? `${parent.path}.${parent.lastKey}`
          : parent.lastKey
        : "";
      stack.push({ keys: new Set(), path, lastKey: "" });
    } else if (ch === "}") {
      stack.pop();
    }
    i++;
  }
  return duplicates;
}

describe("i18n source coverage", () => {
  it.each(["en", "es"])("%s locale files have no duplicate keys", (locale) => {
    for (const file of listLocaleFiles(locale)) {
      const raw = readFileSync(file, "utf8");
      expect(findDuplicateJsonKeys(raw), file).toEqual([]);
    }
  });

  it.each(["en", "es"])("%s namespace files do not redefine keys from other files", (locale) => {
    const owners = new Map<string, string>();
    const overlaps: string[] = [];
    for (const file of listLocaleFiles(locale)) {
      const keys = leafKeys(JSON.parse(readFileSync(file, "utf8")));
      for (const key of keys) {
        const owner = owners.get(key);
        if (owner) overlaps.push(`${key} (${owner} & ${file})`);
        owners.set(key, file);
      }
    }
    expect(overlaps).toEqual([]);
  });

  it("every static t() key used in source exists in en.json", () => {
    const missing: string[] = [];
    for (const file of listSourceFiles(SRC_DIR)) {
      const source = readFileSync(file, "utf8");
      for (const match of source.matchAll(STATIC_KEY_PATTERN)) {
        if (typeof lookup(en, match[1]) !== "string") missing.push(`${file}: ${match[1]}`);
      }
    }
    expect(missing).toEqual([]);
  });

  it("components contain no hardcoded user-facing text", () => {
    const hardcoded: string[] = [];
    for (const file of listSourceFiles(SRC_DIR).filter((f) => f.endsWith(".svelte"))) {
      const markup = readFileSync(file, "utf8")
        .replace(/<script[\s\S]*?<\/script>/g, "")
        .replace(/<style[\s\S]*?<\/style>/g, "")
        .replace(/<!--[\s\S]*?-->/g, "");
      for (const match of markup.matchAll(MARKUP_TEXT_PATTERN)) {
        if (match[1].trim()) hardcoded.push(`${file}: ${match[1].trim()}`);
      }
      for (const match of markup.matchAll(MARKUP_ATTR_PATTERN)) {
        hardcoded.push(`${file}: ${match[0]}`);
      }
    }
    expect(hardcoded).toEqual([]);
  });
});
