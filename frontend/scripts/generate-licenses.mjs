import { execSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = join(dirname(fileURLToPath(import.meta.url)), "..");

const raw = execSync(
  "bunx license-checker-rseidelsohn --production --excludePrivatePackages --json",
  { cwd: frontendDir },
);

const packages = JSON.parse(raw.toString());

const lines = [
  "",
  "## Frontend Dependencies (npm)",
  "",
  "| Package | License |",
  "|---------|---------|",
];

for (const [name, info] of Object.entries(packages).sort(([a], [b]) =>
  a.localeCompare(b),
)) {
  const license = info.licenses ?? "Unknown";
  const repo = info.repository;
  const cell = repo ? `[${license}](${repo})` : license;
  lines.push(`| ${name} | ${cell} |`);
}

console.log(lines.join("\n"));
