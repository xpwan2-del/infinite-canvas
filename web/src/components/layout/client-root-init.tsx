"use client";

import type { ReactNode } from "react";
import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { App } from "antd";

import { useConfigStore } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

export function ClientRootInit({ children }: { children: ReactNode }) {
    const { message } = App.useApp();
    const handledConfigParams = useRef(false);
    const pathname = usePathname();
    const hydrateUser = useUserStore((state) => state.hydrateUser);
    const loadPublicSettings = useConfigStore((state) => state.loadPublicSettings);
    const loadTopAIModels = useConfigStore((state) => state.loadTopAIModels);
    const publicSettings = useConfigStore((state) => state.publicSettings);
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const openConfigDialog = useConfigStore((state) => state.openConfigDialog);

    useEffect(() => {
        void loadPublicSettings();
    }, [loadPublicSettings]);

    useEffect(() => {
        void hydrateUser().then(async () => {
            if (!useUserStore.getState().user) redirectToTopAILogin();
            else {
                try {
                    await loadPublicSettings();
                    await loadTopAIModels(useUserStore.getState().token);
                } catch (error) {
                    message.error(error instanceof Error ? error.message : "读取 TOP-AI 模型失败");
                }
            }
        });
    }, [hydrateUser, loadPublicSettings, loadTopAIModels, message]);

    useEffect(() => {
        if (handledConfigParams.current) return;
        const searchParams = new URLSearchParams(window.location.search);
        const baseUrl = searchParams.get("baseUrl") || searchParams.get("baseurl");
        const apiKey = searchParams.get("apiKey") || searchParams.get("apikey");
        if (!baseUrl && !apiKey) return;
        if (!publicSettings) return;
        handledConfigParams.current = true;
        searchParams.delete("baseUrl");
        searchParams.delete("baseurl");
        searchParams.delete("apiKey");
        searchParams.delete("apikey");
        window.history.replaceState(null, "", `${window.location.pathname}${searchParams.size ? `?${searchParams}` : ""}${window.location.hash}`);
        if (!publicSettings.modelChannel.allowCustomChannel) {
            openConfigDialog(false);
            message.error("后台未允许用户自定义渠道，请联系管理员进行配置");
            return;
        }
        updateConfig("channelMode", "local");
        if (baseUrl) updateConfig("baseUrl", baseUrl);
        if (apiKey) updateConfig("apiKey", apiKey);
        openConfigDialog(false);
    }, [message, openConfigDialog, publicSettings, updateConfig]);

    return <>{children}</>;
}

function redirectToTopAILogin() {
    const configuredPath = process.env.NEXT_PUBLIC_TOP_AI_LOGIN_PATH || "/login";
    const redirect = `${window.location.pathname}${window.location.search}${window.location.hash}`;
    const target = new URL(configuredPath, window.location.origin);
    target.searchParams.set("redirect", redirect);
    window.location.href = target.origin === window.location.origin ? `${target.pathname}${target.search}${target.hash}` : target.toString();
}
