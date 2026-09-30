using System.Diagnostics;
using System.Net;
using System.Net.Http;
using System.Net.Sockets;
using Connla.Desktop;

if (args.Length != 2) throw new ArgumentException("Pass the backend EXE and an isolated test parent directory.");
var backend = Path.GetFullPath(args[0]);
var root = Path.Combine(Path.GetFullPath(args[1]), $"backup-flow-{Guid.NewGuid():N}");
var data = Path.Combine(root, "data");
var outputParent = Path.Combine(root, "external-backups");
Directory.CreateDirectory(outputParent);
Directory.CreateDirectory(data);
var portListener = new TcpListener(IPAddress.Loopback, 0);
portListener.Start();
var port = ((IPEndPoint)portListener.LocalEndpoint).Port;
portListener.Stop();

async Task CheckBootAsync()
{
    var start = new ProcessStartInfo(backend) { UseShellExecute = false, CreateNoWindow = true };
    foreach (var argument in new[] { "--addr", "127.0.0.1", "--port", port.ToString(), "--data", data }) start.ArgumentList.Add(argument);
    using var server = Process.Start(start) ?? throw new Exception("Server did not start");
    try
    {
        using var client = new HttpClient { Timeout = TimeSpan.FromSeconds(1) };
        var ready = false;
        for (var attempt = 0; attempt < 80; attempt++)
        {
            if (server.HasExited) throw new Exception("Server exited early");
            try { ready = (await client.GetAsync($"http://127.0.0.1:{port}/healthz")).IsSuccessStatusCode; }
            catch (HttpRequestException) { }
            catch (TaskCanceledException) { }
            if (ready) break;
            await Task.Delay(250);
        }
        if (!ready) throw new Exception("Server did not become healthy");
    }
    finally
    {
        if (!server.HasExited) server.Kill(entireProcessTree: true);
        server.WaitForExit(10000);
    }
}

void Check(bool condition, string message)
{
    if (!condition) throw new Exception(message);
}

await CheckBootAsync();
Check(File.Exists(Path.Combine(data, "memos_prod.db")), "Fixture database missing");
var operations = new BackupOperations(backend, data);
var backup = await operations.CreateAsync(outputParent, CancellationToken.None);
Check(File.Exists(Path.Combine(backup, "manifest.json")), "Backup manifest missing");
var rejectedDataFolder = false;
try { await operations.CreateAsync(data, CancellationToken.None); }
catch (InvalidOperationException) { rejectedDataFolder = true; }
Check(rejectedDataFolder, "Backup accepted a destination inside live data");
File.WriteAllText(Path.Combine(data, "original-marker.txt"), "must survive rollback");

var tampered = Path.Combine(root, "tampered-backup");
foreach (var file in Directory.GetFiles(backup, "*", SearchOption.AllDirectories))
{
    var copy = Path.Combine(tampered, Path.GetRelativePath(backup, file));
    Directory.CreateDirectory(Path.GetDirectoryName(copy)!);
    File.Copy(file, copy);
}
File.AppendAllText(Path.Combine(tampered, "memos_prod.db"), "tampered");
var rejectedTampering = false;
try { await operations.PrepareRestoreAsync(tampered, CancellationToken.None); }
catch (InvalidOperationException) { rejectedTampering = true; }
Check(rejectedTampering && File.Exists(Path.Combine(data, "original-marker.txt")), "Tampered backup changed live data");

var prepared = await operations.PrepareRestoreAsync(backup, CancellationToken.None);
Check(File.Exists(Path.Combine(prepared.SafetyBackupDirectory, "manifest.json")), "Safety backup missing");
var swap = new RestoreSwap(data);
swap.Switch(prepared);
Check(!File.Exists(Path.Combine(data, "original-marker.txt")), "Restore did not switch datasets");
Check(File.Exists(Path.Combine(prepared.OriginalDirectory, "original-marker.txt")), "Previous data was not preserved");
var failed = swap.RollBack(prepared);
Check(File.Exists(Path.Combine(data, "original-marker.txt")), "Rollback did not restore previous data");
Check(File.Exists(Path.Combine(failed, "memos_prod.db")), "Failed restore was not preserved");

prepared = await operations.PrepareRestoreAsync(backup, CancellationToken.None);
swap.Switch(prepared);
swap.Complete();
await CheckBootAsync();
Check(!File.Exists(Path.Combine(data, "original-marker.txt")), "Successful restore did not remain active");

prepared = await operations.PrepareRestoreAsync(backup, CancellationToken.None);
swap.Switch(prepared);
var interruptedData = Path.Combine(root, "interrupted-restored-data");
Directory.Move(data, interruptedData);
swap.RecoverIfNeeded();
Check(File.Exists(Path.Combine(data, "memos_prod.db")), "Interrupted restore did not recover previous data");
await CheckBootAsync();

File.WriteAllText(Path.Combine(data, "recovery-marker.txt"), "must survive interrupted restart");
prepared = await operations.PrepareRestoreAsync(backup, CancellationToken.None);
swap.Switch(prepared);
swap.RecoverIfNeeded();
Check(File.Exists(Path.Combine(data, "recovery-marker.txt")), "Interrupted restore with new data present did not recover previous data");

Console.WriteLine("PASS: verified backup, restore, tamper rejection, safety copy, rollback, interrupted-restore recovery, and server boot");
Console.WriteLine($"Isolated test directory: {root}");
