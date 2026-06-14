export const APP_BASE_PATH = normalizeBasePath(process.env.NEXT_PUBLIC_CANVAS_BASE_PATH || "");

export function withBasePath(path: string) {
    if (!APP_BASE_PATH || !path.startsWith("/")) return path;
    if (path === APP_BASE_PATH || path.startsWith(`${APP_BASE_PATH}/`)) return path;
    return `${APP_BASE_PATH}${path}`;
}

export function normalizeBasePath(value: string) {
    const trimmed = value.trim();
    if (!trimmed || trimmed === "/") return "";
    return `/${trimmed.replace(/^\/+|\/+$/g, "")}`;
}
