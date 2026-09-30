using System.Diagnostics;

namespace Connla.Desktop;

// The bundled Go command owns backup format, file hashing and SQLite checks.
// This class only chooses new directories and invokes that verified offline workflow.
internal sealed class BackupOperations(string serverPath, string dataDirectory)
{
    public string DataDirectory { get; } = Path.GetFullPath(dataDirectory);

    public string BackupRoot => Path.Combine(Path.GetDirectoryName(DataDirectory)!, "backups");

    public async Task<string> CreateAsync(string destinationParent, CancellationToken cancellationToken)
    {
        var parent = Path.GetFullPath(destinationParent);
        if (IsInside(parent, DataDirectory))
            throw new InvalidOperationException("备份位置不能选在 Connla 数据目录内。请另选文件夹或磁盘。");

        var destination = NewPath(parent, "Connla-backup");
        await RunCommandAsync(["backup", "create", "--data", DataDirectory, "--output", destination], cancellationToken);
        return destination;
    }

    public async Task<PreparedRestore> PrepareRestoreAsync(string backupDirectory, CancellationToken cancellationToken)
    {
        var source = Path.GetFullPath(backupDirectory);
        if (IsInside(source, DataDirectory))
            throw new InvalidOperationException("请选择数据目录之外的备份文件夹。");
        if (!File.Exists(Path.Combine(source, "manifest.json")))
            throw new InvalidOperationException("所选文件夹不是 Connla 备份：缺少 manifest.json。");

        // A restore never starts until the currently active data has its own verified backup.
        Directory.CreateDirectory(BackupRoot);
        var safetyBackup = NewPath(BackupRoot, "pre-restore-backup");
        await RunCommandAsync(["backup", "create", "--data", DataDirectory, "--output", safetyBackup], cancellationToken);

        var stagingDirectory = NewPath(Path.GetDirectoryName(DataDirectory)!, "data-restoring");
        await RunCommandAsync(["backup", "restore", "--input", source, "--target", stagingDirectory], cancellationToken);
        return new PreparedRestore(stagingDirectory, NewPath(BackupRoot, "pre-restore-original"), safetyBackup);
    }

    public async Task RunCommandAsync(IEnumerable<string> arguments, CancellationToken cancellationToken)
    {
        var start = new ProcessStartInfo(serverPath)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
        };
        foreach (var argument in arguments) start.ArgumentList.Add(argument);
        using var process = new Process { StartInfo = start };
        if (!process.Start()) throw new InvalidOperationException("无法启动 Connla 备份组件。");
        var stdout = process.StandardOutput.ReadToEndAsync(cancellationToken);
        var stderr = process.StandardError.ReadToEndAsync(cancellationToken);
        try { await process.WaitForExitAsync(cancellationToken); }
        catch (OperationCanceledException)
        {
            if (!process.HasExited) process.Kill(entireProcessTree: true);
            throw;
        }
        var output = await stdout;
        var error = await stderr;
        if (process.ExitCode != 0)
        {
            var detail = string.IsNullOrWhiteSpace(error) ? output : error;
            throw new InvalidOperationException($"备份组件未能完成操作：{detail.Trim()[..Math.Min(detail.Trim().Length, 600)]}");
        }
    }

    public static bool IsInside(string candidate, string root)
    {
        var relative = Path.GetRelativePath(Path.GetFullPath(root), Path.GetFullPath(candidate));
        return relative == "." || !Path.IsPathFullyQualified(relative) && relative != ".." &&
            !relative.StartsWith(".." + Path.DirectorySeparatorChar, StringComparison.Ordinal);
    }

    private static string NewPath(string parent, string prefix) =>
        Path.Combine(parent, $"{prefix}-{DateTime.Now:yyyyMMdd-HHmmss}-{Guid.NewGuid():N}");
}

internal sealed record PreparedRestore(string StagingDirectory, string OriginalDirectory, string SafetyBackupDirectory);
