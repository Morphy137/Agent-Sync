const WEB_PROTOCOLS = new Set(["http:", "https:"]);
const SCP_LIKE_SSH = /^[\w.-]+@([\w.-]+):(?!\/)(.+?)(?:\.git)?\/?$/;
const GIT_SUFFIX = /(?:\.git)?\/?$/;

/**
 * Web page of a repository, or null when the URL has none we can safely open.
 * http(s) URLs are used as they are (minus credentials and a trailing ".git"); SSH URLs
 * (`git@host:owner/repo.git`, `ssh://git@host/owner/repo.git`) map to their https page.
 */
export function repositoryPageUrl(url: string): string | null {
  const value = url.trim();
  const scp = SCP_LIKE_SSH.exec(value);
  if (scp) return `https://${scp[1]}/${scp[2]}`;

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return null;
  }
  if (parsed.protocol === "ssh:" && parsed.hostname) {
    return `https://${parsed.hostname}${parsed.pathname.replace(GIT_SUFFIX, "")}`;
  }
  if (!WEB_PROTOCOLS.has(parsed.protocol)) return null;
  parsed.username = "";
  parsed.password = "";
  parsed.search = "";
  parsed.hash = "";
  parsed.pathname = parsed.pathname.replace(GIT_SUFFIX, "");
  return parsed.toString();
}
