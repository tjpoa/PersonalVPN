namespace PersonalVpn.Client;

public sealed record WindowsProvisioningResult(WireGuardIdentity Identity, DeviceResponse Device, IReadOnlyList<RegionResponse> Regions);

/// Coordinates local identity with the authenticated control-plane API.
public sealed class VpnProvisioningCoordinator
{
    private readonly PersonalVpnApiClient _api;
    private readonly WireGuardIdentityStore _identityStore;

    public VpnProvisioningCoordinator(PersonalVpnApiClient api, WireGuardIdentityStore identityStore)
    {
        _api = api;
        _identityStore = identityStore;
    }

    public async Task<WindowsProvisioningResult> EstablishAsync(string deviceName, CancellationToken cancellationToken)
    {
        if (string.IsNullOrWhiteSpace(deviceName) || deviceName.Length > 128)
            throw new ArgumentException("Invalid device name", nameof(deviceName));
        var identity = _identityStore.LoadOrCreate();
        var devices = await _api.ListDevicesAsync(cancellationToken).ConfigureAwait(false);
        var device = devices.FirstOrDefault(candidate => candidate.Platform == "windows" && candidate.PublicKey == identity.PublicKey)
            ?? await _api.RegisterDeviceAsync(new RegisterDeviceRequest(deviceName, "windows", identity.PublicKey), cancellationToken).ConfigureAwait(false);
        var regions = await _api.ListRegionsAsync(cancellationToken).ConfigureAwait(false);
        return new WindowsProvisioningResult(identity, device, regions);
    }
}
