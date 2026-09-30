using System.Text.Json;
using Microsoft.Web.WebView2.Core;

namespace Connla.Desktop;

internal sealed partial class MainWindow
{
    private async Task StartOwnedServerAsync()
    {
        if (_serverPath is null) throw new InvalidOperationException("Connla 服务组件尚未准备好。");
        if (!CanBindLoopback(_options.Port)) throw new InvalidOperationException($"本机端口 {_options.Port} 已被占用，Connla 无法启动。");
        var server = StartServer(_serverPath, _options);
        _server = server;
        server.EnableRaisingEvents = true;
        server.Exited += (_, _) =>
        {
            if (_closing.IsCancellationRequested || _maintenanceBusy || !ReferenceEquals(server, _server) || IsDisposed || !IsHandleCreated) return;
            BeginInvoke(() =>
            {
                if (_closing.IsCancellationRequested || _maintenanceBusy || !ReferenceEquals(server, _server) || IsDisposed) return;
                MessageBox.Show("Connla 本地服务已退出。请查看数据目录中的 server.err.log。", "Connla", MessageBoxButtons.OK, MessageBoxIcon.Error);
                Environment.ExitCode = 1;
                Close();
            });
        };
        try { await WaitForServerAsync(server, _options.Port, _closing.Token); }
        catch
        {
            StopServerForMaintenance();
            throw;
        }
    }

    private void StopServerForMaintenance()
    {
        var server = _server;
        if (server is null) return;
        if (!server.HasExited)
        {
            try { server.Kill(entireProcessTree: true); }
            catch (InvalidOperationException) when (server.HasExited) { }
            if (!server.WaitForExit(10000)) throw new InvalidOperationException("本地服务没有按时停止，未进行数据操作。");
        }
        _server = null;
        server.Dispose();
    }

    private async void OnWebMessageReceived(object? sender, CoreWebView2WebMessageReceivedEventArgs e)
    {
        // JavaScript cannot pass a filesystem path. Only the visible Settings page
        // may request the native picker, and the native side confirms restores.
        if (!IsLocalUri(e.Source) || !Uri.TryCreate(e.Source, UriKind.Absolute, out var source) ||
            source.AbsolutePath != "/setting") return;

        string? requestId = null;
        string? action = null;
        try
        {
            using var document = JsonDocument.Parse(e.WebMessageAsJson);
            if (document.RootElement.ValueKind == JsonValueKind.Object &&
                document.RootElement.TryGetProperty("type", out var type) && type.ValueKind == JsonValueKind.String &&
                type.GetString() == "connla:maintenance" &&
                document.RootElement.TryGetProperty("id", out var id) && id.ValueKind == JsonValueKind.String &&
                document.RootElement.TryGetProperty("action", out var requestedAction) && requestedAction.ValueKind == JsonValueKind.String)
            {
                requestId = id.GetString();
                action = requestedAction.GetString();
            }
        }
        catch (JsonException) { return; }
        if (requestId is null || requestId.Length is < 1 or > 100 || action is not ("backup" or "restore")) return;
        if (_maintenanceBusy)
        {
            PostMaintenanceResult(requestId, "error", "已有备份或恢复操作正在进行，请稍后重试。");
            return;
        }

        try { await RunMaintenanceAsync(requestId, action); }
        catch (Exception error) { PostMaintenanceResult(requestId, "error", error.Message); }
    }

    private async Task RunMaintenanceAsync(string requestId, string action)
    {
        using var picker = new FolderBrowserDialog
        {
            Description = action == "backup" ? "选择保存 Connla 备份的文件夹" : "选择含 manifest.json 的 Connla 备份文件夹",
            ShowNewFolderButton = action == "backup",
            UseDescriptionForTitle = true,
        };
        if (picker.ShowDialog(this) != DialogResult.OK)
        {
            PostMaintenanceResult(requestId, "cancelled", "已取消。");
            return;
        }
        if (action == "restore")
        {
            var confirmed = MessageBox.Show(this,
                $"即将使用以下备份恢复知识库：\n{picker.SelectedPath}\n\n恢复前会先校验备份并保留当前数据的副本。操作期间 Connla 会短暂停止。是否继续？",
                "确认恢复知识库", MessageBoxButtons.YesNo, MessageBoxIcon.Warning, MessageBoxDefaultButton.Button2);
            if (confirmed != DialogResult.Yes)
            {
                PostMaintenanceResult(requestId, "cancelled", "已取消。");
                return;
            }
        }

        _maintenanceBusy = true;
        _status.Text = action == "backup" ? "正在校验并备份数据…" : "正在校验备份并恢复数据…";
        _status.Visible = true;
        _status.BringToFront();
        var result = "error";
        var message = "操作未完成。";
        try
        {
            StopServerForMaintenance();
            var backup = new BackupOperations(_serverPath!, _options.DataDirectory);
            if (action == "backup")
            {
                var path = await backup.CreateAsync(picker.SelectedPath, _closing.Token);
                await StartOwnedServerAsync();
                result = "success";
                message = $"备份已校验并保存到：{path}";
            }
            else
            {
                var prepared = await backup.PrepareRestoreAsync(picker.SelectedPath, _closing.Token);
                var swap = new RestoreSwap(_options.DataDirectory);
                swap.Switch(prepared);
                try
                {
                    await StartOwnedServerAsync();
                    swap.Complete();
                    result = "success";
                    message = $"恢复完成。恢复前的数据和安全备份保存在：{backup.BackupRoot}";
                }
                catch (Exception startError)
                {
                    StopServerForMaintenance();
                    swap.RollBack(prepared);
                    await StartOwnedServerAsync();
                    throw new InvalidOperationException("恢复后的数据无法启动，已自动回退到原数据。" + startError.Message, startError);
                }
            }
        }
        catch (Exception error)
        {
            message = error.Message;
            if (_server is null || _server.HasExited)
            {
                try
                {
                    StopServerForMaintenance();
                    await StartOwnedServerAsync();
                }
                catch (Exception restartError)
                {
                    message += " 原数据未被删除，但服务未能重启：" + restartError.Message;
                    Environment.ExitCode = 1;
                }
            }
        }
        finally
        {
            _status.Visible = false;
            _maintenanceBusy = false;
        }

        PostMaintenanceResult(requestId, result, message);
        if (result == "success" && action == "restore")
        {
            MessageBox.Show(this, message + "\n\n点击确定后将重新加载界面；如恢复了其他账号的数据，可能需要重新登录。", "Connla", MessageBoxButtons.OK, MessageBoxIcon.Information);
            _webView.Reload();
        }
        else if (_server is null)
        {
            MessageBox.Show(this, message, "Connla 无法继续运行", MessageBoxButtons.OK, MessageBoxIcon.Error);
            Close();
        }
    }

    private void PostMaintenanceResult(string requestId, string status, string message)
    {
        if (_webView.CoreWebView2 is null || IsDisposed) return;
        _webView.CoreWebView2.PostWebMessageAsJson(JsonSerializer.Serialize(new
        {
            type = "connla:maintenance-result",
            id = requestId,
            status,
            message,
        }));
    }
}
