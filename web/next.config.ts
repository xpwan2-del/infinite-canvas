import type { NextConfig } from "next";
import { PHASE_DEVELOPMENT_SERVER } from "next/constants";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { parseChangelog } from "@/lib/release";

const webDir = dirname(fileURLToPath(import.meta.url));
const localVersion = readFileSync(resolve(webDir, "../VERSION"), "utf8").trim() || "dev";
const localChangelog = readFileSync(resolve(webDir, "../CHANGELOG.md"), "utf8");

function normalizeBasePath(value: string | undefined) {
    const trimmed = (value || "").trim();
    if (!trimmed || trimmed === "/") return "";
    return `/${trimmed.replace(/^\/+|\/+$/g, "")}`;
}

export default function nextConfig(phase: string): NextConfig {
    const isDev = phase === PHASE_DEVELOPMENT_SERVER;
    const releases = parseChangelog(localChangelog);
    const canvasBasePath = normalizeBasePath(process.env.CANVAS_BASE_PATH || process.env.NEXT_PUBLIC_CANVAS_BASE_PATH);

    return {
        output: "standalone",
        ...(canvasBasePath ? { basePath: canvasBasePath } : {}),
        allowedDevOrigins: isDev ? ["*.*.*.*"] : [],
        typescript: {
            ignoreBuildErrors: true,
        },
        env: {
            NEXT_PUBLIC_CANVAS_BASE_PATH: canvasBasePath,
            NEXT_PUBLIC_APP_VERSION: localVersion,
            NEXT_PUBLIC_APP_RELEASES: JSON.stringify(releases),
        },
    };
}
