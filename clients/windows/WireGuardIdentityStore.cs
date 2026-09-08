using System.Text.Json;

namespace PersonalVpn.Client;

public sealed record WireGuardIdentity(string PrivateKey, string PublicKey);

/// Persists the device identity encrypted with the current user's DPAPI key.
public sealed class WireGuardIdentityStore
{
    private readonly DpapiKeyStore _keyStore;
    private readonly WireGuardTool _tool;
    private readonly string _path;

    public WireGuardIdentityStore(DpapiKeyStore keyStore, WireGuardTool tool, string? path = null)
    {
        _keyStore = keyStore;
        _tool = tool;
        _path = path ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "PersonalVPN", "wireguard-identity.bin");
    }

    public WireGuardIdentity LoadOrCreate()
    {
        try
        {
            if (File.Exists(_path))
            {
                var identity = JsonSerializer.Deserialize<WireGuardIdentity>(_keyStore.Unprotect(File.ReadAllBytes(_path)))
                    ?? throw new InvalidOperationException("Stored WireGuard identity is empty");
                if (identity.PrivateKey.Length != 44 || identity.PublicKey.Length != 44 || _tool.DerivePublicKey(identity.PrivateKey) != identity.PublicKey)
                    throw new InvalidOperationException("Stored WireGuard identity is invalid");
                return identity;
            }
        }
        catch (Exception exception) { throw new InvalidOperationException("Could not load WireGuard identity", exception); }

        var privateKey = _tool.GeneratePrivateKey();
        var created = new WireGuardIdentity(privateKey, _tool.DerivePublicKey(privateKey));
        var directory = Path.GetDirectoryName(_path) ?? throw new InvalidOperationException("Invalid identity path");
        Directory.CreateDirectory(directory);
        File.WriteAllBytes(_path, _keyStore.Protect(JsonSerializer.SerializeToUtf8Bytes(created)));
        return created;
    }
}
