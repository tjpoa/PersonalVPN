using System.Text.Json;

namespace PersonalVpn.Client;

public sealed class AuthTokenStore
{
    private readonly DpapiKeyStore _keyStore;
    private readonly string _path;

    public AuthTokenStore(DpapiKeyStore keyStore, string? path = null)
    {
        _keyStore = keyStore;
        _path = path ?? Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "PersonalVPN", "session.bin");
    }

    public void Save(TokenPair tokens)
    {
        if (!IsSessionToken(tokens.AccessToken) || !IsSessionToken(tokens.RefreshToken))
            throw new ArgumentException("Invalid token pair", nameof(tokens));
        var directory = Path.GetDirectoryName(_path) ?? throw new InvalidOperationException("Invalid token path");
        Directory.CreateDirectory(directory);
        var payload = JsonSerializer.SerializeToUtf8Bytes(tokens);
        File.WriteAllBytes(_path, _keyStore.Protect(payload));
    }

    public TokenPair? Load()
    {
        try
        {
            if (!File.Exists(_path)) return null;
            var payload = _keyStore.Unprotect(File.ReadAllBytes(_path));
            var tokens = JsonSerializer.Deserialize<TokenPair>(payload);
            return tokens is not null && IsSessionToken(tokens.AccessToken) && IsSessionToken(tokens.RefreshToken) ? tokens : null;
        }
        catch (Exception)
        {
            return null;
        }
    }

    public void Clear()
    {
        if (File.Exists(_path)) File.Delete(_path);
    }

    private static bool IsSessionToken(string value) => value is { Length: 47 } && value.StartsWith("pv1_", StringComparison.Ordinal);
}
