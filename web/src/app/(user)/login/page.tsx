"use client";

import { ArrowRight, Loader2 } from "lucide-react";
import { Button } from "antd";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { withBasePath } from "@/lib/base-path";
import { useUserStore } from "@/stores/use-user-store";

function safeRedirect(value: string | null, fallback: string): string {
    const cleaned = (value ?? "").replace(/[\t\n\r]/g, "");
    if (!cleaned.startsWith("/") || cleaned.startsWith("//") || cleaned.startsWith("/\\")) {
        return fallback;
    }
    return cleaned;
}

export default function LoginPage() {
    return (
        <Suspense fallback={null}>
            <LoginBridge />
        </Suspense>
    );
}

function LoginBridge() {
    const router = useRouter();
    const searchParams = useSearchParams();
    const user = useUserStore((state) => state.user);
    const hydrateTopAISession = useUserStore((state) => state.hydrateTopAISession);
    const [checked, setChecked] = useState(false);
    const redirect = safeRedirect(searchParams.get("redirect"), withBasePath("/"));
    const logoMask = `url(${withBasePath("/logo.svg")}) center / contain no-repeat`;

    useEffect(() => {
        void hydrateTopAISession().finally(() => setChecked(true));
    }, [hydrateTopAISession]);

    useEffect(() => {
        if (!user) return;
        router.replace(redirect);
        router.refresh();
    }, [redirect, router, user]);

    return (
        <main className="flex h-full min-h-0 items-center justify-center overflow-y-auto bg-background bg-[radial-gradient(#e5e7eb_1px,transparent_1px)] px-6 py-10 [background-size:16px_16px] dark:bg-[radial-gradient(rgba(245,245,244,.16)_1px,transparent_1px)]">
            <section className="w-full max-w-[420px] text-center">
                <span
                    className="mx-auto mb-4 block size-12 bg-stone-950 dark:bg-stone-100"
                    style={{
                        mask: logoMask,
                        WebkitMask: logoMask,
                    }}
                    aria-label="无限画布"
                />
                <h1 className="text-3xl font-semibold tracking-normal text-stone-950 dark:text-stone-100">使用 TOP-AI 登录</h1>
                <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">画布使用 TOP-AI 账号进入，余额、模型和计费由主平台统一管理。</p>
                <Button type="primary" size="large" className="mt-7" icon={checked ? <ArrowRight className="size-4" /> : <Loader2 className="size-4 animate-spin" />} onClick={() => redirectToTopAILogin(redirect)}>
                    前往 TOP-AI 登录
                </Button>
            </section>
        </main>
    );
}

function redirectToTopAILogin(redirect: string) {
    const configuredPath = process.env.NEXT_PUBLIC_TOP_AI_LOGIN_PATH || "/login";
    const target = new URL(configuredPath, window.location.origin);
    target.searchParams.set("redirect", redirect);
    window.location.href = target.origin === window.location.origin ? `${target.pathname}${target.search}${target.hash}` : target.toString();
}
