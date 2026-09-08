using System.Diagnostics;

namespace PersonalVpn.Client;

/// Invokes the signed WireGuard CLI for key operations; no WireGuard crypto is reimplemented here.
public sealed class WireGuardTool
{
    private readonly string _executable;

    public WireGuardTool(string? executable = null)
    {
        _executable = executable ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "WireGuard", "wg.exe");
        if (!File.Exists(_executable)) throw new FileNotFoundException("Official WireGuard wg.exe was not found", _executable);
    }

    public string GeneratePrivateKey() => Run("genkey", null);

    public string DerivePublicKey(string privateKey)
    {
        if (!IsKey(privateKey)) throw new ArgumentException("Invalid WireGuard private key", nameof(privateKey));
        return Run("pubkey", privateKey + Environment.NewLine);
    }

    private string Run(string arguments, string? standardInput)
    {
        using var process = Process.Start(new ProcessStartInfo(_executable, arguments)
        {
            RedirectStandardInput = true, RedirectStandardOutput = true, RedirectStandardError = true,
            UseShellExecute = false, CreateNoWindow = true
        }) ?? throw new InvalidOperationException("Could not start WireGuard CLI");
        if (standardInput is not null) process.StandardInput.Write(standardInput);
        process.StandardInput.Close();
        var output = process.StandardOutput.ReadToEnd().Trim();
        var error = process.StandardError.ReadToEnd().Trim();
        process.WaitForExit();
        if (process.ExitCode != 0 || !IsKey(output)) throw new InvalidOperationException($"WireGuard CLI failed: {error}");
        return output;
    }

    private static bool IsKey(string value)
    {
        try { return Convert.FromBase64String(value.Trim()).Length == 32; }
        catch (FormatException) { return false; }
    }
}
