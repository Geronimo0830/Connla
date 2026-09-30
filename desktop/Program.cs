using System.Diagnostics;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using System.Reflection;
using System.Security.Cryptography;
using Microsoft.Web.WebView2.Core;
using Microsoft.Web.WebView2.WinForms;

namespace Connla.Desktop;

internal static class Program
{
    [STAThread]
    private static void Main(string[] args)
    {
        ApplicationConfiguration.Initialize();

        try
        {
            var options = DesktopOptions.Parse(args);
            var dataKey = Convert.ToHexString(SHA256.HashData(System.Text.Encoding.UTF8.GetBytes(options.DataDirectory.ToUpperInvariant())))[..16];
            using var mutex = new Mutex(true, $"Local\\ConnlaDesktop-{dataKey}", out var firstInstance);
            if (!firstInstance)
            {
                MessageBox.Show("Connla 桌面版已在运行。", "Connla", MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }

            Application.Run(new MainWindow(options));
        }
        catch (Exception error)
        {
            MessageBox.Show(error.Message, "Connla 无法启动", MessageBoxButtons.OK, MessageBoxIcon.Error);
            Environment.ExitCode = 1;
        }
    }
}

internal sealed record DesktopOptions(string DataDirectory, int Port, bool SmokeTest)
{
    public static DesktopOptions Parse(string[] args)
    {
        var dataDirectory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "PersonalKnowledgeBase", "data");
        var port = 8081;
        var smokeTest = false;

        for (var index = 0; index < args.Length; index++)
        {
            switch (args[index])
            {
                case "--data":
                    if (++index >= args.Length) throw new ArgumentException("--data 缺少目录路径。");
                    dataDirectory = Path.GetFullPath(args[index]);
                    break;
                case "--port":
                    if (++index >= args.Length || !int.TryParse(args[index], out port) || port is < 1 or > 65535)
                        throw new ArgumentException("--port 必须是 1 到 65535 之间的数字。");
                    break;
                case "--smoke-test":
                    smokeTest = true;
                    break;
                default:
                    throw new ArgumentException($"无法识别的参数：{args[index]}");
            }
        }

        return new DesktopOptions(dataDirectory, port, smokeTest);
    }
}

internal sealed partial class MainWindow : Form
{
    private readonly DesktopOptions _options;
    private readonly Label _status;
    private readonly WebView2 _webView;
    private readonly CancellationTokenSource _closing = new();
    private Process? _server;
    private string? _serverPath;
    private bool _maintenanceBusy;
    private Uri? _localOrigin;

    public MainWindow(DesktopOptions options)
    {
        _options = options;
        Text = "Connla";
        Width = 1280;
        Height = 820;
        MinimumSize = new Size(900, 600);
        StartPosition = FormStartPosition.CenterScreen;
        BackColor = Color.FromArgb(29, 31, 35);
        ForeColor = Color.White;
        try { Icon = Icon.ExtractAssociatedIcon(Application.ExecutablePath); } catch { /* The window still works without a shell icon. */ }

        _status = new Label
        {
            Dock = DockStyle.Fill,
            Text = "正在启动 Connla…",
            TextAlign = ContentAlignment.MiddleCenter,
            Font = new Font("Microsoft YaHei UI", 13),
        };
        _webView = new WebView2 { Dock = DockStyle.Fill, Visible = false };
        Controls.Add(_webView);
        Controls.Add(_status);
        Shown += async (_, _) => await StartAsync();
        FormClosing += (_, e) =>
        {
            if (_maintenanceBusy)
            {
                e.Cancel = true;
                MessageBox.Show("正在备份或恢复，请等待操作完成后再关闭窗口。", "Connla", MessageBoxButtons.OK, MessageBoxIcon.Information);
                return;
            }
            _closing.Cancel();
            StopOwnedServer();
        };
    }

    private async Task StartAsync()
    {
        try
        {
            if (!CanBindLoopback(_options.Port))
                throw new InvalidOperationException($"本机端口 {_options.Port} 已被占用。请先关闭旧版 Connla，再打开桌面版；不会自动结束其他程序。");
            var restoreSwap = new RestoreSwap(_options.DataDirectory);
            restoreSwap.RecoverIfNeeded();

            CoreWebView2Environment.GetAvailableBrowserVersionString();
            _serverPath = ExtractServer();
            Directory.CreateDirectory(_options.DataDirectory);
            await StartOwnedServerAsync();
            restoreSwap.Complete();

            _localOrigin = new Uri($"http://127.0.0.1:{_options.Port}/");
            var profileDirectory = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Connla", "WebView2");
            var environment = await CoreWebView2Environment.CreateAsync(userDataFolder: profileDirectory);
            await _webView.EnsureCoreWebView2Async(environment);
            _webView.CoreWebView2.WebMessageReceived += OnWebMessageReceived;
            _webView.CoreWebView2.NavigationStarting += (_, e) =>
            {
                if (!IsLocalUri(e.Uri))
                {
                    e.Cancel = true;
                    OpenExternalUri(e.Uri);
                }
            };
            _webView.CoreWebView2.NewWindowRequested += (_, e) =>
            {
                e.Handled = true;
                if (IsLocalUri(e.Uri)) _webView.CoreWebView2.Navigate(e.Uri);
                else OpenExternalUri(e.Uri);
            };
            _webView.CoreWebView2.Settings.AreDevToolsEnabled = false;
            _webView.NavigationCompleted += (_, e) =>
            {
                if (!_options.SmokeTest) return;
                Environment.ExitCode = e.IsSuccess ? 0 : 1;
                Close();
            };

            _status.Visible = false;
            _webView.Visible = true;
            _webView.Source = _localOrigin;
        }
        catch (OperationCanceledException) when (_closing.IsCancellationRequested)
        {
            // The user closed the window while the backend was starting.
        }
        catch (Exception error)
        {
            var details = error is WebView2RuntimeNotFoundException
                ? "需要 Microsoft Edge WebView2 Runtime。请从微软官网安装后重试。"
                : error.Message;
            MessageBox.Show(details, "Connla 无法启动", MessageBoxButtons.OK, MessageBoxIcon.Error);
            Environment.ExitCode = 1;
            Close();
        }
    }

    private bool IsLocalUri(string rawUri)
    {
        if (_localOrigin is null || !Uri.TryCreate(rawUri, UriKind.Absolute, out var uri)) return false;
        return uri.Scheme == Uri.UriSchemeHttp && uri.Host == IPAddress.Loopback.ToString() && uri.Port == _localOrigin.Port;
    }

    private static void OpenExternalUri(string rawUri)
    {
        if (!Uri.TryCreate(rawUri, UriKind.Absolute, out var uri)) return;
        if (uri.Scheme is not ("http" or "https")) return;
        try { Process.Start(new ProcessStartInfo(uri.AbsoluteUri) { UseShellExecute = true }); } catch { /* Invalid external links stay blocked. */ }
    }

    private static bool CanBindLoopback(int port)
    {
        try
        {
            var listener = new TcpListener(IPAddress.Loopback, port);
            listener.Start();
            listener.Stop();
            return true;
        }
        catch (SocketException) { return false; }
    }

    private static string ExtractServer()
    {
        using var resource = Assembly.GetExecutingAssembly().GetManifestResourceStream("Connla.Server.exe")
            ?? throw new InvalidOperationException("程序内缺少 Connla 服务组件，请重新下载完整版本。");
        var runtimeRoot = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "Connla", "Runtime");
        Directory.CreateDirectory(runtimeRoot);
        var temporaryPath = Path.Combine(runtimeRoot, $"{Guid.NewGuid():N}.tmp");
        try
        {
            using (var output = File.Create(temporaryPath)) resource.CopyTo(output);
            var expectedHash = HashFile(temporaryPath);
            var versionDirectory = Path.Combine(runtimeRoot, expectedHash[..16]);
            Directory.CreateDirectory(versionDirectory);
            var serverPath = Path.Combine(versionDirectory, "ConnlaServer.exe");
            if (File.Exists(serverPath))
            {
                var actualHash = HashFile(serverPath);
                if (!actualHash.Equals(expectedHash, StringComparison.Ordinal))
                    throw new InvalidOperationException("本地 Connla 服务组件校验失败，请检查数据目录或重新下载程序。");
            }
            else File.Move(temporaryPath, serverPath);
            return serverPath;
        }
        finally
        {
            if (File.Exists(temporaryPath)) File.Delete(temporaryPath);
        }
    }

    private static string HashFile(string path)
    {
        using var file = File.OpenRead(path);
        return Convert.ToHexString(SHA256.HashData(file));
    }

    private static Process StartServer(string serverPath, DesktopOptions options)
    {
        var start = new ProcessStartInfo(serverPath)
        {
            UseShellExecute = false,
            CreateNoWindow = true,
            RedirectStandardOutput = true,
            RedirectStandardError = true,
        };
        foreach (var argument in new[] { "--addr", "127.0.0.1", "--port", options.Port.ToString(), "--data", options.DataDirectory })
            start.ArgumentList.Add(argument);
        var server = new Process { StartInfo = start };
        server.OutputDataReceived += (_, e) => AppendLog(options.DataDirectory, "server.out.log", e.Data);
        server.ErrorDataReceived += (_, e) => AppendLog(options.DataDirectory, "server.err.log", e.Data);
        if (!server.Start()) throw new InvalidOperationException("Connla 服务未能启动。");
        server.BeginOutputReadLine();
        server.BeginErrorReadLine();
        return server;
    }

    private static void AppendLog(string dataDirectory, string fileName, string? line)
    {
        if (line is null) return;
        try { File.AppendAllText(Path.Combine(dataDirectory, fileName), line + Environment.NewLine); } catch { /* Logging must not crash the app. */ }
    }

    private static async Task WaitForServerAsync(Process server, int port, CancellationToken cancellationToken)
    {
        using var client = new HttpClient { Timeout = TimeSpan.FromSeconds(1) };
        var health = $"http://127.0.0.1:{port}/healthz";
        for (var attempt = 0; attempt < 80; attempt++)
        {
            cancellationToken.ThrowIfCancellationRequested();
            if (server.HasExited) throw new InvalidOperationException("Connla 服务启动失败，请查看数据目录中的 server.err.log。");
            try
            {
                using var response = await client.GetAsync(health, cancellationToken);
                if (response.IsSuccessStatusCode) return;
            }
            catch (HttpRequestException) { }
            catch (TaskCanceledException) when (!cancellationToken.IsCancellationRequested) { }
            await Task.Delay(500, cancellationToken);
        }
        throw new TimeoutException("Connla 服务启动超时，请查看数据目录中的 server.err.log。");
    }

    private void StopOwnedServer()
    {
        try { StopServerForMaintenance(); }
        catch { /* Closing never stops an unrelated process. */ }
    }
}
