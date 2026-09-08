using System.Text;

namespace PersonalVpn.Client;

public static class WireGuardConfigRenderer
{
    public static string Render(string privateKey, VpnConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);
        Validate(configuration);
        if (!IsKey(privateKey))
            throw new ArgumentException("Invalid WireGuard key", nameof(configuration));

        var builder = new StringBuilder()
            .AppendLine("[Interface]")
            .AppendLine($"PrivateKey = {privateKey}")
            .AppendLine($"Address = {string.Join(", ", configuration.Addresses)}")
            .AppendLine($"DNS = {string.Join(", ", configuration.DnsServers)}")
            .AppendLine()
            .AppendLine("[Peer]")
            .AppendLine($"PublicKey = {configuration.ServerPublicKey}")
            .AppendLine($"Endpoint = {configuration.Endpoint}")
            .AppendLine($"AllowedIPs = {string.Join(", ", configuration.AllowedIps)}")
            .AppendLine($"PersistentKeepalive = {configuration.PersistentKeepaliveSeconds}");
        return builder.ToString();
    }

    public static void Validate(VpnConfiguration configuration)
    {
        ArgumentNullException.ThrowIfNull(configuration);
        if (!IsKey(configuration.ServerPublicKey))
            throw new ArgumentException("Invalid WireGuard key", nameof(configuration));
        if (configuration.Addresses is null || configuration.AllowedIps is null || configuration.Addresses.Count == 0 || configuration.AllowedIps.Count == 0)
            throw new ArgumentException("Configuration has no addresses or routes", nameof(configuration));
        if (configuration.PersistentKeepaliveSeconds is < 0 or > 120)
            throw new ArgumentException("Invalid keepalive", nameof(configuration));

        Uri endpoint;
        try { endpoint = new Uri($"udp://{configuration.Endpoint}"); }
        catch (UriFormatException exception) { throw new ArgumentException("Invalid UDP endpoint", nameof(configuration), exception); }
        if (string.IsNullOrEmpty(endpoint.DnsSafeHost) || endpoint.Port is < 1 or > 65535 ||
            !string.IsNullOrEmpty(endpoint.UserInfo) || endpoint.AbsolutePath != "/" ||
            !string.IsNullOrEmpty(endpoint.Query) || !string.IsNullOrEmpty(endpoint.Fragment))
            throw new ArgumentException("Invalid UDP endpoint", nameof(configuration));
    }

    private static bool IsKey(string value)
    {
        try { return Convert.FromBase64String(value).Length == 32; }
        catch (ArgumentException) { return false; }
    }
}
