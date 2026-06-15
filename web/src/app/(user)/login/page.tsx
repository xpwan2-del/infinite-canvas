"use client";

import { ArrowRight, Loader2 } from "lucide-react";
import { Alert, Button, Input, message } from "antd";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, type FormEvent, type ReactNode, useEffect, useState } from "react";

import { withBasePath } from "@/lib/base-path";
import { useConfigStore } from "@/stores/use-config-store";
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
    const isUserLoading = useUserStore((state) => state.isLoading);
    const login = useUserStore((state) => state.login);
    const register = useUserStore((state) => state.register);
    const hydrateTopAISession = useUserStore((state) => state.hydrateTopAISession);
    const publicSettings = useConfigStore((state) => state.publicSettings);
    const isPublicSettingsLoading = useConfigStore((state) => state.isPublicSettingsLoading);
    const loadPublicSettings = useConfigStore((state) => state.loadPublicSettings);
    const [checked, setChecked] = useState(false);
    const [mode, setMode] = useState<"login" | "register">("login");
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [settingsError, setSettingsError] = useState("");
    const redirect = safeRedirect(searchParams.get("redirect"), withBasePath("/"));
    const logoMask = `url(${withBasePath("/logo.svg")}) center / contain no-repeat`;
    const forceTopAIGateway = publicSettings?.canvas?.forceTopAIGateway === true;
    const allowRegister = publicSettings?.auth?.allowRegister === true;
    const isSettingsReady = Boolean(publicSettings) || Boolean(settingsError);

    useEffect(() => {
        void hydrateTopAISession().finally(() => setChecked(true));
    }, [hydrateTopAISession]);

    useEffect(() => {
        void loadPublicSettings().catch((error) => {
            setSettingsError(error instanceof Error ? error.message : "读取登录配置失败");
        });
    }, [loadPublicSettings]);

    useEffect(() => {
        if (!user) return;
        router.replace(redirect);
        router.refresh();
    }, [redirect, router, user]);

    async function handleSubmit(event: FormEvent<HTMLFormElement>) {
        event.preventDefault();
        const payload = { username: username.trim(), password };
        if (!payload.username || !payload.password) {
            message.warning("请输入账号和密码");
            return;
        }
        try {
            if (mode === "register") await register(payload);
            else await login(payload);
            router.replace(redirect);
            router.refresh();
        } catch (error) {
            message.error(error instanceof Error ? error.message : "登录失败");
        }
    }

    const isBusy = isUserLoading || isPublicSettingsLoading || !isSettingsReady;
    if (!checked || !isSettingsReady) {
        return (
            <LoginShell logoMask={logoMask}>
                <Loader2 className="mx-auto size-8 animate-spin text-stone-500 dark:text-stone-300" />
            </LoginShell>
        );
    }

    if (forceTopAIGateway) {
        return (
            <LoginShell logoMask={logoMask}>
                <h1 className="text-3xl font-semibold tracking-normal text-stone-950 dark:text-stone-100">使用 TOP-AI 登录</h1>
                <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">画布使用 TOP-AI 账号进入，余额、模型和计费由主平台统一管理。</p>
                <Button type="primary" size="large" className="mt-7" icon={isBusy ? <Loader2 className="size-4 animate-spin" /> : <ArrowRight className="size-4" />} onClick={() => redirectToTopAILogin(redirect)}>
                    前往 TOP-AI 登录
                </Button>
            </LoginShell>
        );
    }

    return (
        <LoginShell logoMask={logoMask}>
            <h1 className="text-3xl font-semibold tracking-normal text-stone-950 dark:text-stone-100">{mode === "register" ? "创建画布账号" : "登录无限画布"}</h1>
            <p className="mt-3 text-base leading-7 text-stone-500 dark:text-stone-400">本地调试使用画布账号登录；线上环境可由 TOP-AI 统一登录态接管。</p>
            {settingsError ? <Alert className="mt-5 text-left" type="warning" showIcon message={settingsError} /> : null}
            <form className="mt-7 space-y-4 text-left" onSubmit={handleSubmit}>
                <Input size="large" placeholder="账号" value={username} autoComplete="username" onChange={(event) => setUsername(event.target.value)} />
                <Input.Password size="large" placeholder="密码" value={password} autoComplete={mode === "register" ? "new-password" : "current-password"} onChange={(event) => setPassword(event.target.value)} />
                <Button type="primary" size="large" htmlType="submit" block loading={isUserLoading}>
                    {mode === "register" ? "注册并进入" : "登录"}
                </Button>
            </form>
            {allowRegister ? (
                <Button type="link" className="mt-3" onClick={() => setMode(mode === "register" ? "login" : "register")}>
                    {mode === "register" ? "已有账号，去登录" : "没有账号，注册一个"}
                </Button>
            ) : null}
        </LoginShell>
    );
}

function LoginShell({ children, logoMask }: { children: ReactNode; logoMask: string }) {
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
                {children}
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
