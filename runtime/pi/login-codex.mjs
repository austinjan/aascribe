// Explicit subscription login for A; never print or copy access/refresh tokens.
import { resolve } from "node:path";
import { mkdir, chmod } from "node:fs/promises";
import { dirname } from "node:path";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
const authPath = resolve(process.argv[2] || "data/remote-mcp/pi/auth.json");
const method = process.argv[3] || "browser";
if (!["browser", "device_code"].includes(method)) throw new Error("Use browser or device_code login");
await mkdir(dirname(authPath), { recursive: true });
const runtime = await ModelRuntime.create({ authPath, modelsPath: null, refreshOnCreate: false, allowModelNetwork: false });
try {
  await runtime.login("openai-codex", "oauth", {
    signal: AbortSignal.timeout(10 * 60 * 1000),
    prompt: async (prompt) => {
      if (prompt.type === "select" && prompt.options?.some(option => option.id === method)) return method;
      if (prompt.type === "manual_code") return new Promise((_resolve, reject) => {
        if (prompt.signal?.aborted) reject(new Error("Login cancelled"));
        else prompt.signal?.addEventListener("abort", () => reject(new Error("Login cancelled")), { once: true });
      });
      throw new Error("Unexpected login prompt; use the pi CLI for interactive login");
    },
    notify: (event) => {
      if (event.type === "device_code") console.log(JSON.stringify({ verification_url: event.verificationUri, user_code: event.userCode }));
      else if (event.type === "auth_url") console.log(JSON.stringify({ login_url: event.url }));
    },
  }, { agentName: "aascribe" });
  await chmod(authPath, 0o600);
  console.log(JSON.stringify({ status: "logged_in", provider: "openai-codex", auth_file: authPath }));
} catch {
  console.error("Codex subscription login failed or expired; no credentials were printed.");
  process.exitCode = 1;
}
