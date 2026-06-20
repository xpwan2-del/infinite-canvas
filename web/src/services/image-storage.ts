"use client";

import axios from "axios";
import localforage from "localforage";

import { nanoid } from "nanoid";
import { withBasePath } from "@/lib/base-path";
import { readImageMeta } from "@/lib/image-utils";
import { useUserStore } from "@/stores/use-user-store";

export type UploadedImage = {
    url: string;
    storageKey: string;
    width: number;
    height: number;
    bytes: number;
    mimeType: string;
};

type ApiEnvelope<T> = T | { code?: number; data?: T | null; msg?: string };
type GeneratedMediaUploadResponse = { id: string; url: string; storageKey?: string; mimeType: string; bytes: number };

const store = localforage.createInstance({ name: "infinite-canvas", storeName: "image_files" });
const objectUrls = new Map<string, string>();

export async function uploadImage(input: string | Blob): Promise<UploadedImage> {
    const blob = await blobFromInput(input);
    const storageKey = `image:${nanoid()}`;
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    const meta = await readImageMeta(url);
    return { url, storageKey, width: meta.width, height: meta.height, bytes: blob.size, mimeType: blob.type || meta.mimeType };
}

export async function uploadGeneratedImage(input: string | Blob): Promise<UploadedImage> {
    const token = useUserStore.getState().token;
    if (!token) throw new Error("保存生成图片需要先登录");
    const blob = await blobFromInput(input);
    const body = new FormData();
    body.append("file", blob, generatedImageFileName(blob));
    try {
        const response = await axios.post<ApiEnvelope<GeneratedMediaUploadResponse>>(withBasePath("/api/v1/media/generated"), body, { headers: { Authorization: `Bearer ${token}` } });
        const payload = unwrapEnvelope(response.data, "生成图片保存失败");
        if (!payload.storageKey || !isRemoteR2ImageKey(payload.storageKey)) throw new Error("生成图片保存后没有返回 R2 存储键");
        const url = await setImageBlob(payload.storageKey, blob);
        const meta = await readImageMeta(url);
        return { url, storageKey: payload.storageKey, width: meta.width, height: meta.height, bytes: payload.bytes || blob.size, mimeType: payload.mimeType || blob.type || meta.mimeType };
    } catch (error) {
        throw new Error(readAxiosError(error, "生成图片保存失败"));
    }
}

export async function resolveImageUrl(storageKey?: string, fallback = "") {
    if (!storageKey) return fallback;
    const cached = objectUrls.get(storageKey);
    if (cached) return cached;
    const blob = await store.getItem<Blob>(storageKey);
    if (blob) {
        const url = URL.createObjectURL(blob);
        objectUrls.set(storageKey, url);
        return url;
    }
    if (isRemoteR2ImageKey(storageKey)) return resolveRemoteR2ImageUrl(storageKey, fallback);
    return fallback;
}

export async function getImageBlob(storageKey: string) {
    return store.getItem<Blob>(storageKey);
}

export async function setImageBlob(storageKey: string, blob: Blob) {
    await store.setItem(storageKey, blob);
    const url = URL.createObjectURL(blob);
    objectUrls.set(storageKey, url);
    return url;
}

export async function imageToDataUrl(image: { url?: string; dataUrl?: string; storageKey?: string }) {
    const url = image.dataUrl || (await resolveImageUrl(image.storageKey, image.url || ""));
    if (!url || url.startsWith("data:")) return url;
    return blobToDataUrl(await (await fetch(url)).blob());
}

export async function deleteStoredImages(keys: Iterable<string>) {
    await Promise.all(
        Array.from(new Set(keys)).map(async (key) => {
            const url = objectUrls.get(key);
            if (url) URL.revokeObjectURL(url);
            objectUrls.delete(key);
            await store.removeItem(key);
        }),
    );
}

export async function cleanupUnusedImages(usedData: unknown) {
    const usedKeys = collectImageStorageKeys(usedData);
    const unused: string[] = [];
    await store.iterate((_value, key) => {
        if (!usedKeys.has(key)) unused.push(key);
    });
    await deleteStoredImages(unused);
}

export function collectImageStorageKeys(value: unknown, keys = new Set<string>()) {
    if (!value || typeof value !== "object") return keys;
    if ("storageKey" in value && typeof value.storageKey === "string" && (value.storageKey.startsWith("image:") || isRemoteR2ImageKey(value.storageKey))) keys.add(value.storageKey);
    Object.values(value).forEach((item) => (Array.isArray(item) ? item.forEach((child) => collectImageStorageKeys(child, keys)) : collectImageStorageKeys(item, keys)));
    return keys;
}

async function resolveRemoteR2ImageUrl(storageKey: string, fallback: string) {
    const token = useUserStore.getState().token;
    if (!token) return fallback;
    try {
        const response = await axios.get<ApiEnvelope<{ url?: string }>>(withBasePath("/api/v1/media/generated"), {
            headers: { Authorization: `Bearer ${token}` },
            params: { key: storageKey },
        });
        const payload = response.data;
        if (typeof payload === "object" && payload && "code" in payload && typeof payload.code === "number") return payload.code === 0 ? payload.data?.url || fallback : fallback;
        return "url" in payload ? payload.url || fallback : fallback;
    } catch {
        return fallback;
    }
}

function isRemoteR2ImageKey(storageKey: string) {
    return storageKey.startsWith("r2:generated/");
}

function unwrapEnvelope<T>(payload: ApiEnvelope<T>, emptyMessage: string): T {
    if (!payload) throw new Error(emptyMessage);
    if (typeof payload === "object" && "code" in payload && typeof payload.code === "number") {
        if (payload.code !== 0) throw new Error(payload.msg || "请求失败");
        if (!payload.data) throw new Error(emptyMessage);
        return payload.data;
    }
    return payload as T;
}

function readAxiosError(error: unknown, fallback: string) {
    if (axios.isAxiosError<{ error?: { message?: string }; msg?: string; code?: number }>(error)) {
        const responseData = error.response?.data;
        return responseData?.msg || responseData?.error?.message || readStatusError(error.response?.status, fallback);
    }
    return error instanceof Error ? error.message : fallback;
}

function readStatusError(status: number | undefined, fallback: string) {
    if (status === 401 || status === 403) return "鉴权失败，请重新登录";
    if (status === 413) return "生成图片超过大小限制，请使用 30MB 以内的图片";
    return status ? `${fallback}：${status}` : fallback;
}

async function blobFromInput(input: string | Blob) {
    if (typeof input !== "string") return input;
    if (input.startsWith("data:")) return dataUrlToBlob(input);
    return (await fetch(input)).blob();
}

function dataUrlToBlob(dataUrl: string) {
    const [meta = "", payload = ""] = dataUrl.split(",", 2);
    const mimeType = meta.match(/^data:([^;]+)/)?.[1] || "image/png";
    if (!meta.includes(";base64")) return new Blob([decodeURIComponent(payload)], { type: mimeType });
    const binary = atob(payload);
    const bytes = new Uint8Array(binary.length);
    for (let index = 0; index < binary.length; index += 1) bytes[index] = binary.charCodeAt(index);
    return new Blob([bytes], { type: mimeType });
}

function generatedImageFileName(blob: Blob) {
    return `generated-image-${nanoid()}${imageExtByMimeType(blob.type)}`;
}

function imageExtByMimeType(mimeType: string) {
    switch (mimeType.toLowerCase()) {
        case "image/jpeg":
        case "image/jpg":
            return ".jpg";
        case "image/webp":
            return ".webp";
        case "image/bmp":
            return ".bmp";
        case "image/gif":
            return ".gif";
        case "image/heic":
            return ".heic";
        case "image/heif":
            return ".heif";
        case "image/png":
        default:
            return ".png";
    }
}

function blobToDataUrl(blob: Blob) {
    return new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(String(reader.result || ""));
        reader.onerror = () => reject(new Error("读取图片失败"));
        reader.readAsDataURL(blob);
    });
}
