#!/usr/bin/env node
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, relative, resolve, sep } from "node:path";

const repoRoot = resolve(import.meta.dirname, "..");
const schemaPath = resolve(repoRoot, "packs/pack.schema.json");
const packRoot = resolve(repoRoot, "packs/official");
const failures = [];

function fail(message) {
  failures.push(message);
}

function repoRel(path) {
  return relative(repoRoot, path).split(sep).join("/");
}

function readJSON(path) {
  try {
    return JSON.parse(readFileSync(path, "utf8"));
  } catch (error) {
    fail(`invalid json: ${repoRel(path)}: ${error.message}`);
    return undefined;
  }
}

function isPlainObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function validate(schema, value, path) {
  if (!schema) return;
  if (schema.enum && !schema.enum.includes(value)) {
    fail(`${path} must be one of ${schema.enum.join(", ")}`);
    return;
  }
  if (schema.type) {
    const ok =
      (schema.type === "array" && Array.isArray(value)) ||
      (schema.type === "object" && isPlainObject(value)) ||
      (schema.type === "integer" && Number.isInteger(value)) ||
      (schema.type === "number" && typeof value === "number" && Number.isFinite(value)) ||
      (schema.type === "string" && typeof value === "string") ||
      (schema.type === "boolean" && typeof value === "boolean");
    if (!ok) {
      fail(`${path} must be ${schema.type}`);
      return;
    }
  }
  if (typeof value === "string") {
    if (schema.minLength && value.length < schema.minLength) fail(`${path} must not be empty`);
    if (schema.pattern && !(new RegExp(schema.pattern).test(value))) fail(`${path} does not match ${schema.pattern}`);
  }
  if (typeof value === "number" && schema.minimum !== undefined && value < schema.minimum) {
    fail(`${path} must be >= ${schema.minimum}`);
  }
  if (Array.isArray(value)) {
    if (schema.minItems && value.length < schema.minItems) fail(`${path} must have at least ${schema.minItems} item(s)`);
    if (schema.uniqueItems) {
      const seen = new Set();
      for (const item of value) {
        const key = JSON.stringify(item);
        if (seen.has(key)) fail(`${path} must not contain duplicate items: ${key}`);
        seen.add(key);
      }
    }
    for (const [index, item] of value.entries()) validate(schema.items, item, `${path}[${index}]`);
  }
  if (isPlainObject(value)) {
    const required = schema.required ?? [];
    for (const key of required) {
      if (!(key in value)) fail(`${path}.${key} is required`);
    }
    if (schema.additionalProperties === false) {
      const allowed = new Set(Object.keys(schema.properties ?? {}));
      for (const key of Object.keys(value)) {
        if (!allowed.has(key)) fail(`${path}.${key} is not allowed by schema`);
      }
    }
    for (const [key, childSchema] of Object.entries(schema.properties ?? {})) {
      if (key in value) validate(childSchema, value[key], `${path}.${key}`);
    }
    if (schema.additionalProperties && isPlainObject(schema.additionalProperties)) {
      for (const [key, childValue] of Object.entries(value)) {
        if (!schema.properties || !(key in schema.properties)) {
          validate(schema.additionalProperties, childValue, `${path}.${key}`);
        }
      }
    }
  }
}

const schema = existsSync(schemaPath) ? readJSON(schemaPath) : undefined;
if (!schema) fail("missing or invalid packs/pack.schema.json");
if (!existsSync(packRoot)) fail("missing packs/official");

let manifestCount = 0;
if (schema && existsSync(packRoot)) {
  for (const dirent of readdirSync(packRoot, { withFileTypes: true })) {
    if (!dirent.isDirectory()) continue;
    const manifestPath = join(packRoot, dirent.name, "pack.json");
    if (!existsSync(manifestPath)) {
      fail(`official pack missing manifest: ${repoRel(manifestPath)}`);
      continue;
    }
    manifestCount += 1;
    const manifest = readJSON(manifestPath);
    if (!manifest) continue;
    validate(schema, manifest, repoRel(manifestPath));

    const routes = new Set(manifest.backend?.routes ?? []);
    const routeSpecs = manifest.backend?.routeSpecs ?? [];
    for (const spec of routeSpecs) {
      if (!routes.has(spec.path)) {
        fail(`${repoRel(manifestPath)} backend.routeSpecs path is not listed in backend.routes: ${spec.path}`);
      }
    }
    for (const route of routes) {
      if (!routeSpecs.some((spec) => spec.path === route)) {
        fail(`${repoRel(manifestPath)} backend.routes path lacks a routeSpecs entry: ${route}`);
      }
    }

    const menuPaths = new Set((manifest.frontend?.menus ?? []).map((item) => item.path));
    for (const route of manifest.frontend?.routes ?? []) {
      if (!menuPaths.has(route.path)) {
        fail(`${repoRel(manifestPath)} frontend route is not reachable from a menu path: ${route.path}`);
      }
    }
  }
}

if (failures.length > 0) {
  console.error("Pack manifest schema check failed:");
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log(`Pack manifest schema ok: ${manifestCount} official manifests validated`);
