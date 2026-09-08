using System.Security.Cryptography;

namespace PersonalVpn.Client;

public sealed class DpapiKeyStore
{
    private readonly byte[] _entropy;

    public DpapiKeyStore(byte[] entropy)
    {
        if (entropy.Length < 16) throw new ArgumentException("Entropy must be at least 16 bytes", nameof(entropy));
        _entropy = entropy.ToArray();
    }

    public byte[] ProtectPrivateKey(byte[] privateKey)
    {
        if (privateKey.Length != 32) throw new ArgumentException("WireGuard private key must be 32 bytes", nameof(privateKey));
        return ProtectedData.Protect(privateKey, _entropy, DataProtectionScope.CurrentUser);
    }

    public byte[] Protect(byte[] value)
    {
        if (value.Length == 0) throw new ArgumentException("Value cannot be empty", nameof(value));
        return ProtectedData.Protect(value, _entropy, DataProtectionScope.CurrentUser);
    }

    public byte[] Unprotect(byte[] value)
    {
        if (value.Length == 0) throw new ArgumentException("Value cannot be empty", nameof(value));
        return ProtectedData.Unprotect(value, _entropy, DataProtectionScope.CurrentUser);
    }

    public byte[] UnprotectPrivateKey(byte[] protectedKey)
    {
        var privateKey = ProtectedData.Unprotect(protectedKey, _entropy, DataProtectionScope.CurrentUser);
        if (privateKey.Length != 32)
            throw new CryptographicException("Stored WireGuard private key has an invalid length");
        return privateKey;
    }
}
