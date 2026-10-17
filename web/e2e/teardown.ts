import { execFileSync } from "node:child_process";
export default async function teardown() {
  execFileSync(
    "docker",
    [
      "compose",
      "-p",
      "afterglow-test",
      "-f",
      "compose.test.yaml",
      "down",
      "-v",
    ],
    { cwd: "..", stdio: "inherit" },
  );
}
