const RESERVED_SUBDOMAINS = new Set([
  "www",
  "api",
  "admin",
  "app",
  "portal",
  "console",
  "help",
  "support",
  "status",
  "login",
  "auth",
  "static",
  "assets",
  "cdn",
  "mail",
  "smtp",
  "ftp",
]);

const MAX_LABEL_LENGTH = 63;

export function normalizeSubdomain(name: string): string {
  let normalized = name.normalize("NFKD").toLowerCase();

  normalized = normalized.replaceAll(" ", "-");
  normalized = normalized.replaceAll("_", "-");
  normalized = normalized.replace(/[^a-z0-9-]/g, "");
  normalized = normalized.replace(/-+/g, "-");
  normalized = normalized.replace(/^-+|-+$/g, "");

  if (normalized.length > MAX_LABEL_LENGTH) {
    normalized = normalized.slice(0, MAX_LABEL_LENGTH).replace(/-+$/g, "");
  }

  return normalized || "org";
}

export function validateSubdomain(subdomain: string): string | null {
  if (!subdomain) {
    return "subdomain cannot be empty";
  }

  if (subdomain.length > MAX_LABEL_LENGTH) {
    return "subdomain cannot exceed 63 characters";
  }

  if (subdomain.length < 2) {
    return "subdomain must be at least 2 characters";
  }

  if (!/^[a-z]/.test(subdomain)) {
    return "subdomain must start with a letter";
  }

  if (!/[a-z0-9]$/.test(subdomain)) {
    return "subdomain must end with a letter or number";
  }

  if (!/^[a-z0-9-]+$/.test(subdomain)) {
    return "subdomain can only contain lowercase letters, numbers, and dashes";
  }

  if (RESERVED_SUBDOMAINS.has(subdomain)) {
    return `'${subdomain}' is a reserved subdomain`;
  }

  return null;
}

export function generateSubdomainCandidates(baseName: string, maxCandidates = 5): string[] {
  const base = normalizeSubdomain(baseName);
  const candidates: string[] = [];

  if (validateSubdomain(base) === null) {
    candidates.push(base);
  }

  for (let suffix = 1; candidates.length < maxCandidates && suffix <= 1000; suffix += 1) {
    const suffixPart = `-${suffix}`;
    const maxBaseLen = MAX_LABEL_LENGTH - suffixPart.length;
    const truncatedBase = base.slice(0, Math.max(1, maxBaseLen)).replace(/-+$/g, "");
    const candidate = `${truncatedBase}${suffixPart}`;

    if (validateSubdomain(candidate) === null) {
      candidates.push(candidate);
    }
  }

  return candidates;
}

export function extractSubdomain(host: string, baseDomain: string): string {
  const hostWithoutPort = host.split(":")[0] || "";
  const baseWithoutPort = baseDomain.split(":")[0] || "";

  if (!hostWithoutPort || !baseWithoutPort || hostWithoutPort === baseWithoutPort) {
    return "";
  }

  const suffix = `.${baseWithoutPort}`;
  if (!hostWithoutPort.endsWith(suffix)) {
    return "";
  }

  return hostWithoutPort.slice(0, hostWithoutPort.length - suffix.length);
}

export function isBaseDomain(host: string, baseDomain: string): boolean {
  return extractSubdomain(host, baseDomain) === "";
}

export function isCustomerBaseDomainRedirectPath(pathname: string): boolean {
  return pathname === "/" || pathname === "/login" || pathname === "/login/" || pathname.startsWith("/installs");
}

export function shouldRedirectBaseDomainCustomerRoute(host: string, baseDomain: string, pathname: string): boolean {
  return isBaseDomain(host, baseDomain) && isCustomerBaseDomainRedirectPath(pathname);
}