import { execFileSync } from "node:child_process";
import { randomBytes } from "node:crypto";
export default async function setup() {
  process.env.PLEX_FIXTURE_TOKEN = randomBytes(24).toString("hex");
  execFileSync(
    "docker",
    [
      "compose",
      "-p",
      "afterglow-test",
      "-f",
      "compose.test.yaml",
      "up",
      "-d",
      "--wait",
    ],
    { cwd: "..", stdio: "inherit" },
  );
}
