const DEFAULT_API_BASE_URL = "http://localhost:8080";

const rawApiBaseUrl = import.meta.env.VITE_API_BASE_URL;

function trimTrailingSlashes(value: string): string {
  return value.replace(/\/+$/, "");
}

function resolveApiBaseUrl(): string {
  const candidate = rawApiBaseUrl?.trim();

  if (!candidate) {
    return DEFAULT_API_BASE_URL;
  }

  try {
    const parsed = new URL(candidate);
    return trimTrailingSlashes(parsed.toString());
  } catch {
    throw new Error("VITE_API_BASE_URL must be a valid absolute URL.");
  }
}

export const appConfig = {
  apiBaseUrl: resolveApiBaseUrl()
};
