import { apiGet } from "@/services/api/request";

export type CanvasModelList = {
    models: string[];
    textModels: string[];
    imageModels: string[];
    videoModels: string[];
    audioModels: string[];
};

export async function fetchCanvasModels(token: string) {
    return apiGet<CanvasModelList>("/api/v1/models", undefined, token);
}
