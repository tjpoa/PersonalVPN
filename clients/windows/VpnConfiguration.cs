namespace PersonalVpn.Client;

public sealed record VpnConfiguration(
    string Endpoint,
    string ServerPublicKey,
    IReadOnlyList<string> Addresses,
    IReadOnlyList<string> DnsServers,
    IReadOnlyList<string> AllowedIps,
    int PersistentKeepaliveSeconds);

public interface IWireGuardBackend
{
    Task StartAsync(VpnConfiguration configuration, CancellationToken cancellationToken);
    Task StopAsync(CancellationToken cancellationToken);
}

public sealed class WireGuardNtBackend : IWireGuardBackend
{
    public Task StartAsync(VpnConfiguration configuration, CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(configuration);
        if (string.IsNullOrWhiteSpace(configuration.Endpoint) || string.IsNullOrWhiteSpace(configuration.ServerPublicKey))
            throw new ArgumentException("Invalid VPN configuration", nameof(configuration));
        // The implementation will use the official WireGuard manager service IPC.
        throw new NotSupportedException("WireGuardNT manager IPC is not configured");
    }

    public Task StopAsync(CancellationToken cancellationToken) => Task.CompletedTask;
}
