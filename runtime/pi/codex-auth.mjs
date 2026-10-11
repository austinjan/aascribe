// Reuse Codex's managed login without copying or rotating its refresh token in pi.
import { readFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';
import { createInterface } from 'node:readline';

export function parseCodexAccess(data, now = Date.now()) {
  if (data.auth_mode !== 'chatgpt' || typeof data.tokens?.access_token !== 'string') throw new Error('Codex ChatGPT login is required');
  const access = data.tokens.access_token;
  let claims;
  try { claims = JSON.parse(Buffer.from(access.split('.')[1], 'base64url').toString('utf8')); }
  catch { throw new Error('Invalid Codex access token'); }
  const expires = claims.exp * 1000;
  if (!Number.isFinite(expires)) throw new Error('Codex token has no expiry');
  return { access, expires, needsRefresh: expires < now + 5 * 60 * 1000 };
}

async function readAccess(path) {
  const bytes = await readFile(path);
  if (bytes.length > 64 * 1024) throw new Error('Codex auth file is too large');
  return parseCodexAccess(JSON.parse(bytes.toString('utf8')));
}

async function refreshManagedLogin(command) {
  // No threads or inference turns are started; the CLI alone manages token refresh.
  const child = spawn(command, ['app-server', '--listen', 'stdio://'], { windowsHide: true, stdio: ['pipe', 'pipe', 'ignore'] });
  const lines = createInterface({ input: child.stdout });
  let readRequest = false;
  let total = 0;
  try {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('Codex auth refresh timed out')), 20_000);
      const finish = (error) => { clearTimeout(timer); error ? reject(error) : resolve(); };
      child.once('error', () => finish(new Error('Codex CLI could not start')));
      child.once('exit', () => finish(new Error('Codex CLI exited before auth refresh')));
      lines.on('line', line => {
        total += line.length;
        if (total > 256 * 1024) return finish(new Error('Codex auth response exceeded limit'));
        let message;
        try { message = JSON.parse(line); } catch { return; }
        if (message.id === 1 && !readRequest) {
          if (message.error) return finish(new Error('Codex initialization failed'));
          readRequest = true;
          child.stdin.write(JSON.stringify({method:'initialized'}) + '\n');
          child.stdin.write(JSON.stringify({id:2, method:'account/read', params:{refreshToken:true}}) + '\n');
        } else if (message.id === 2) {
          if (message.error || message.result?.account?.type !== 'chatgpt') return finish(new Error('Codex ChatGPT login is unavailable'));
          finish();
        }
      });
      child.stdin.on('error', () => finish(new Error('Codex auth transport failed')));
      child.stdin.write(JSON.stringify({id:1, method:'initialize', params:{clientInfo:{name:'aascribe_auth', title:'aascribe authentication', version:'0.1.0'}}}) + '\n');
    });
  } finally { lines.close(); child.kill(); }
}

export async function loadCodexAccess(path, command) {
  let credential = await readAccess(path);
  if (credential.needsRefresh) {
    await refreshManagedLogin(command);
    credential = await readAccess(path);
    if (credential.needsRefresh) throw new Error('Codex token was not refreshed; sign in with Codex again');
  }
  // The refresh token stays with Codex; pi gets only this request's access token.
  return {type:'oauth', access:credential.access, refresh:'', expires:credential.expires};
}
