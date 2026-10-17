import { execFileSync } from "node:child_process";
export default async function setup() {
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
