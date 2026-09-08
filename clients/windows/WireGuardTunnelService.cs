using System.Diagnostics;
using System.Security.AccessControl;
using System.Security.Principal;

namespace PersonalVpn.Client;

/// Installs/removes a tunnel through the signed WireGuard manager executable.
/// Callers must run this adapter from an elevated, separately secured service.
public sealed class WireGuardTunnelService
{
    private readonly string _wireguardExe;
    private readonly string _configDirectory;

    public WireGuardTunnelService(string? wireguardExe = null, string? configDirectory = null)
    {
        _wireguardExe = wireguardExe ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "WireGuard", "wireguard.exe");
        _configDirectory = configDirectory ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "PersonalVPN", "tunnels");
        if (!File.Exists(_wireguardExe)) throw new FileNotFoundException("Official WireGuard manager was not found", _wireguardExe);
    }

    public void Install(string tunnelName, string privateKey, VpnConfiguration configuration)
    {
        ValidateName(tunnelName);
        var path = ConfigPath(tunnelName);
        Directory.CreateDirectory(_configDirectory);
        File.WriteAllText(path, WireGuardConfigRenderer.Render(privateKey, configuration));
        RestrictToCurrentUser(path);
        try { Run($"/installtunnelservice \"{path}\""); }
        catch { File.Delete(path); throw; }
    }

    public void Uninstall(string tunnelName)
    {
        ValidateName(tunnelName);
        Run($"/uninstalltunnelservice {tunnelName}");
        var path = ConfigPath(tunnelName);
        if (File.Exists(path)) File.Delete(path);
    }

    private string ConfigPath(string name) => Path.Combine(_configDirectory, name + ".conf");

    private void Run(string arguments)
    {
        using var process = Process.Start(new ProcessStartInfo(_wireguardExe, arguments) { UseShellExecute = false, CreateNoWindow = true })
            ?? throw new InvalidOperationException("Could not start WireGuard manager");
        process.WaitForExit();
        if (process.ExitCode != 0) throw new InvalidOperationException($"WireGuard manager failed with exit code {process.ExitCode}");
    }

    private static void ValidateName(string value)
    {
        if (string.IsNullOrWhiteSpace(value) || value.Length > 64 || !value.All(c => char.IsLetterOrDigit(c) || c is '-' or '_'))
            throw new ArgumentException("Invalid tunnel name", nameof(value));
    }

    private static void RestrictToCurrentUser(string path)
    {
        var sid = WindowsIdentity.GetCurrent().User ?? throw new InvalidOperationException("Current user SID unavailable");
        var security = new FileSecurity();
        security.SetAccessRuleProtection(true, false);
        security.AddAccessRule(new FileSystemAccessRule(sid, FileSystemRights.FullControl, AccessControlType.Allow));
        new FileInfo(path).SetAccessControl(security);
    }
}
