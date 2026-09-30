export type MaintenanceAction = "backup" | "restore";

export interface MaintenanceResult {
  status: "success" | "cancelled" | "error";
  message: string;
}

interface DesktopWebView {
  postMessage(message: unknown): void;
  addEventListener(type: "message", listener: (event: MessageEvent) => void): void;
  removeEventListener(type: "message", listener: (event: MessageEvent) => void): void;
}

const getWebView = (): DesktopWebView | undefined =>
  typeof window === "undefined" ? undefined : (window as Window & { chrome?: { webview?: DesktopWebView } }).chrome?.webview;

export const isDesktopHost = (): boolean => Boolean(getWebView());

export const requestDesktopMaintenance = (action: MaintenanceAction): Promise<MaintenanceResult> => {
  const bridge = getWebView();
  if (!bridge) return Promise.reject(new Error("仅 Connla 桌面版支持整库备份与恢复。"));

  const id = crypto.randomUUID();
  return new Promise((resolve, reject) => {
    const onMessage = (event: MessageEvent) => {
      const data = event.data as Record<string, unknown> | null;
      if (!data || data.type !== "connla:maintenance-result" || data.id !== id) return;
      if (data.status !== "success" && data.status !== "cancelled" && data.status !== "error") return;
      bridge.removeEventListener("message", onMessage);
      resolve({ status: data.status, message: typeof data.message === "string" ? data.message : "" });
    };
    bridge.addEventListener("message", onMessage);
    try {
      bridge.postMessage({ type: "connla:maintenance", id, action });
    } catch (error) {
      bridge.removeEventListener("message", onMessage);
      reject(error);
    }
  });
};
