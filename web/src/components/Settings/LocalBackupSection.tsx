import { DownloadIcon, RotateCcwIcon } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { type MaintenanceAction, requestDesktopMaintenance } from "@/lib/desktop-maintenance";
import { useTranslate } from "@/utils/i18n";
import SettingGroup from "./SettingGroup";
import SettingRow from "./SettingRow";
import SettingSection from "./SettingSection";

const LocalBackupSection = () => {
  const t = useTranslate();
  const [busy, setBusy] = useState<MaintenanceAction | null>(null);
  const [result, setResult] = useState<{ status: "success" | "error"; message: string } | null>(null);

  const run = async (action: MaintenanceAction) => {
    setBusy(action);
    setResult(null);
    try {
      const response = await requestDesktopMaintenance(action);
      if (response.status !== "cancelled") setResult({ status: response.status, message: response.message });
    } catch (error) {
      setResult({ status: "error", message: error instanceof Error ? error.message : t("setting.local-backup.failed") });
    } finally {
      setBusy(null);
    }
  };

  return (
    <SettingSection title={t("setting.local-backup.label")}>
      <SettingGroup>
        <SettingRow label={t("setting.local-backup.backup-title")} description={t("setting.local-backup.backup-description")}>
          <Button type="button" disabled={busy !== null} onClick={() => void run("backup")}>
            <DownloadIcon aria-hidden="true" />
            {busy === "backup" ? t("setting.local-backup.backing-up") : t("setting.local-backup.backup-action")}
          </Button>
        </SettingRow>
        <SettingRow label={t("setting.local-backup.restore-title")} description={t("setting.local-backup.restore-description")}>
          <Button
            type="button"
            variant="outline"
            className="text-destructive hover:text-destructive"
            disabled={busy !== null}
            onClick={() => void run("restore")}
          >
            <RotateCcwIcon aria-hidden="true" />
            {busy === "restore" ? t("setting.local-backup.restoring") : t("setting.local-backup.restore-action")}
          </Button>
        </SettingRow>
      </SettingGroup>
      {busy && (
        <p role="status" className="text-sm text-muted-foreground">
          {busy === "backup" ? t("setting.local-backup.backing-up") : t("setting.local-backup.restoring")}
        </p>
      )}
      {result && (
        <p
          role={result.status === "error" ? "alert" : "status"}
          className={result.status === "error" ? "text-sm text-destructive" : "text-sm text-foreground"}
        >
          {result.message}
        </p>
      )}
    </SettingSection>
  );
};

export default LocalBackupSection;
