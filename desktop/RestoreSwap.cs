namespace Connla.Desktop;

// Keep both the verified snapshot and the previous directory. A small marker
// lets the next launch recover if Windows stops the app between the two moves.
internal sealed class RestoreSwap(string dataDirectory)
{
    private readonly string _dataDirectory = Path.GetFullPath(dataDirectory);
    private string Parent => Path.GetDirectoryName(_dataDirectory)!;
    private string Marker => Path.Combine(Parent, $".{Path.GetFileName(_dataDirectory)}-restore-pending");

    public void RecoverIfNeeded()
    {
        if (!File.Exists(Marker)) return;
        var name = File.ReadAllText(Marker).Trim();
        if (!name.StartsWith("pre-restore-original-", StringComparison.Ordinal) ||
            name != Path.GetFileName(name))
            throw new InvalidOperationException("恢复标记无效；为保护数据，Connla 没有启动。");
        var original = Path.Combine(Parent, "backups", name);
        if (!Directory.Exists(original))
            throw new InvalidOperationException("恢复中断且原数据目录不在预期位置；为保护数据，Connla 没有启动。");
        if (Directory.Exists(_dataDirectory))
        {
            var interrupted = Path.Combine(Parent, "backups", $"interrupted-restore-{DateTime.Now:yyyyMMdd-HHmmss}-{Guid.NewGuid():N}");
            Directory.Move(_dataDirectory, interrupted);
        }
        Directory.Move(original, _dataDirectory);
        File.Delete(Marker);
    }

    public void Switch(PreparedRestore prepared)
    {
        if (!Directory.Exists(_dataDirectory) || !Directory.Exists(prepared.StagingDirectory))
            throw new InvalidOperationException("原数据或已校验的待恢复数据不存在，恢复已取消。");
        if (File.Exists(Marker))
            throw new InvalidOperationException("上一次恢复仍待确认，不能继续恢复。");
        if (!BackupOperations.IsInside(prepared.OriginalDirectory, Path.Combine(Parent, "backups")) ||
            !BackupOperations.IsInside(prepared.StagingDirectory, Parent))
            throw new InvalidOperationException("恢复目录不在预期位置，恢复已取消。");

        var temporaryMarker = Marker + ".tmp";
        File.WriteAllText(temporaryMarker, Path.GetFileName(prepared.OriginalDirectory));
        File.Move(temporaryMarker, Marker);
        try
        {
            Directory.Move(_dataDirectory, prepared.OriginalDirectory);
            Directory.Move(prepared.StagingDirectory, _dataDirectory);
        }
        catch
        {
            if (!Directory.Exists(_dataDirectory) && Directory.Exists(prepared.OriginalDirectory))
                Directory.Move(prepared.OriginalDirectory, _dataDirectory);
            if (Directory.Exists(_dataDirectory)) File.Delete(Marker);
            throw;
        }
    }

    public string RollBack(PreparedRestore prepared)
    {
        if (!Directory.Exists(prepared.OriginalDirectory))
            throw new InvalidOperationException("无法找到恢复前的原数据，未执行自动回退。");
        var failedDirectory = Path.Combine(Parent, "backups", $"failed-restore-{DateTime.Now:yyyyMMdd-HHmmss}-{Guid.NewGuid():N}");
        if (Directory.Exists(_dataDirectory)) Directory.Move(_dataDirectory, failedDirectory);
        Directory.Move(prepared.OriginalDirectory, _dataDirectory);
        File.Delete(Marker);
        return failedDirectory;
    }

    public void Complete()
    {
        if (Directory.Exists(_dataDirectory) && File.Exists(Marker)) File.Delete(Marker);
    }
}
