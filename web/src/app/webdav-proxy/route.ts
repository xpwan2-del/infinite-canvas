import { NextRequest } from "next/server";
import dns from "node:dns/promises";
import net from "node:net";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

const WEBDAV_PROXY_TIMEOUT_MS = 120000;
const WEBDAV_PROXY_MAX_BYTES = Number(process.env.WEBDAV_PROXY_MAX_BYTES || 100 * 1024 * 1024);

export async function POST(request: NextRequest) {
    const authError = await validateCanvasSession(request);
    if (authError) return authError;

    const target = request.headers.get("x-webdav-target") || "";
    const method = (request.headers.get("x-webdav-method") || "GET").toUpperCase();
    if (!target) return new Response("Missing x-webdav-target", { status: 400 });

    let url: URL;
    try {
        url = new URL(target);
    } catch {
        return new Response("Invalid x-webdav-target", { status: 400 });
    }
    if (url.protocol !== "http:" && url.protocol !== "https:") return new Response("Unsupported WebDAV target", { status: 400 });
    if (url.username || url.password) return new Response("WebDAV target credentials are not allowed in URL", { status: 400 });
    const targetError = await validateOutboundTarget(url);
    if (targetError) return targetError;
    const destinationError = validateDestinationHeader(request, url);
    if (destinationError) return destinationError;

    const headers = new Headers();
    copyHeader(request, headers, "x-webdav-authorization", "Authorization");
    copyHeader(request, headers, "x-webdav-depth", "Depth");
    copyHeader(request, headers, "x-webdav-destination", "Destination");
    copyHeader(request, headers, "x-webdav-overwrite", "Overwrite");
    copyHeader(request, headers, "x-webdav-content-type", "Content-Type");

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), WEBDAV_PROXY_TIMEOUT_MS);
    try {
        if (!hasAllowedBodySize(request)) return new Response("WebDAV proxy body too large", { status: 413 });
        const body = method === "GET" || method === "HEAD" ? undefined : await request.arrayBuffer();
        if (body && body.byteLength > WEBDAV_PROXY_MAX_BYTES) return new Response("WebDAV proxy body too large", { status: 413 });
        console.log(`[webdav-proxy] ${method} ${url.host} ${body?.byteLength || 0}B`);
        const response = await fetch(url, { method, headers, body: body?.byteLength ? body : undefined, signal: controller.signal });
        console.log(`[webdav-proxy] ${method} ${url.host} -> ${response.status}`);
        return new Response(method === "HEAD" ? null : response.body, {
            status: response.status,
            headers: responseHeaders(response.headers),
        });
    } catch (error) {
        if (error instanceof Error && error.name === "AbortError") return new Response("WebDAV proxy timeout", { status: 504 });
        return new Response(error instanceof Error ? error.message : "WebDAV proxy error", { status: 502 });
    } finally {
        clearTimeout(timer);
    }
}

async function validateCanvasSession(request: NextRequest) {
    const authorization = request.headers.get("x-canvas-authorization") || "";
    if (!authorization.startsWith("Bearer ")) return new Response("Unauthorized", { status: 401 });
    const apiBaseUrl = (process.env.API_BASE_URL || process.env.CANVAS_API_BASE_URL || "http://127.0.0.1:8080").replace(/\/+$/, "");
    try {
        const response = await fetch(`${apiBaseUrl}/api/auth/me`, {
            headers: { Authorization: authorization, Accept: "application/json" },
            cache: "no-store",
        });
        if (!response.ok) return new Response("Unauthorized", { status: 401 });
        const payload = await response.json();
        if (payload?.code !== 0 || !payload?.data || payload.data.role === "guest") return new Response("Unauthorized", { status: 401 });
        return null;
    } catch {
        return new Response("Canvas auth unavailable", { status: 503 });
    }
}

async function validateOutboundTarget(url: URL) {
    const host = url.hostname.toLowerCase();
    if (isBlockedHost(host)) return new Response("Blocked WebDAV target", { status: 400 });
    if (!isAllowedHost(host)) return new Response("WebDAV target host is not allowed", { status: 403 });

    const directIP = net.isIP(host);
    let addresses: { address: string }[];
    try {
        addresses = directIP ? [{ address: host }] : await dns.lookup(host, { all: true });
    } catch {
        return new Response("WebDAV target host cannot be resolved", { status: 400 });
    }
    if (!addresses.length || addresses.some((item) => isPrivateAddress(item.address))) {
        return new Response("Blocked WebDAV target", { status: 400 });
    }
    return null;
}

function validateDestinationHeader(request: NextRequest, target: URL) {
    const destination = request.headers.get("x-webdav-destination");
    if (!destination) return null;
    try {
        const url = new URL(destination);
        if (url.hostname.toLowerCase() !== target.hostname.toLowerCase()) {
            return new Response("WebDAV destination host must match target host", { status: 400 });
        }
    } catch {
        return new Response("Invalid WebDAV destination", { status: 400 });
    }
    return null;
}

function hasAllowedBodySize(request: NextRequest) {
    const contentLength = Number(request.headers.get("content-length") || 0);
    return !contentLength || contentLength <= WEBDAV_PROXY_MAX_BYTES;
}

function isAllowedHost(host: string) {
    const values = (process.env.WEBDAV_PROXY_ALLOWED_HOSTS || "")
        .split(",")
        .map((item) => item.trim().toLowerCase())
        .filter(Boolean);
    if (!values.length) return true;
    return values.some((value) => host === value || host.endsWith("." + value.replace(/^\./, "")));
}

function isBlockedHost(host: string) {
    return host === "localhost" || host.endsWith(".localhost") || host === "metadata.google.internal" || host.endsWith(".metadata.google.internal");
}

function isPrivateAddress(address: string) {
    if (address.startsWith("::ffff:")) {
        return isPrivateAddress(address.slice("::ffff:".length));
    }
    if (net.isIP(address) === 6) {
        const value = address.toLowerCase();
        return value === "::1" || value.startsWith("fc") || value.startsWith("fd") || value.startsWith("fe80:");
    }
    const parts = address.split(".").map((item) => Number(item));
    if (parts.length !== 4 || parts.some((item) => Number.isNaN(item))) return true;
    const [a, b] = parts;
    return a === 0 || a === 10 || a === 127 || (a === 169 && b === 254) || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 100 && b >= 64 && b <= 127);
}

function copyHeader(request: NextRequest, headers: Headers, from: string, to: string) {
    const value = request.headers.get(from);
    if (value) headers.set(to, value);
}

function responseHeaders(headers: Headers) {
    const result = new Headers();
    ["content-type", "etag", "last-modified", "dav"].forEach((key) => {
        const value = headers.get(key);
        if (value) result.set(key, value);
    });
    return result;
}
