using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace PersonalVpn.Client;

public sealed record RegisterDeviceRequest(
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("platform")] string Platform,
    [property: JsonPropertyName("publicKey")] string PublicKey);

public sealed record AuthCredentials(
    [property: JsonPropertyName("email")] string Email,
    [property: JsonPropertyName("password")] string Password);

public sealed record RegistrationResponse([property: JsonPropertyName("userId")] Guid UserId);

public sealed record TokenPair(
    [property: JsonPropertyName("accessToken")] string AccessToken,
    [property: JsonPropertyName("refreshToken")] string RefreshToken,
    [property: JsonPropertyName("accessExpiresAt")] DateTimeOffset AccessExpiresAt,
    [property: JsonPropertyName("refreshExpiresAt")] DateTimeOffset RefreshExpiresAt);

public sealed record DeviceResponse(
    [property: JsonPropertyName("id")] Guid Id,
    [property: JsonPropertyName("name")] string Name,
    [property: JsonPropertyName("platform")] string Platform,
    [property: JsonPropertyName("publicKey")] string PublicKey,
    [property: JsonPropertyName("state")] string State);

public sealed record RegionResponse(
    [property: JsonPropertyName("id")] string Id,
    [property: JsonPropertyName("displayName")] string DisplayName,
    [property: JsonPropertyName("available")] bool Available);

public sealed record ConfigurationRequest([property: JsonPropertyName("regionId")] string RegionId);

public sealed class PersonalVpnApiClient(HttpClient httpClient, Uri baseUri, string accessToken)
{
    private readonly HttpClient _httpClient = httpClient;
    private readonly Uri _baseUri = RequireHttps(baseUri);
    private readonly string _accessToken = RequireToken(accessToken);

    public static async Task<RegistrationResponse> RegisterAsync(HttpClient httpClient, Uri baseUri, string email, string password, CancellationToken cancellationToken)
    {
        ValidateCredentials(email, password);
        using var response = await SendPublicAsync(httpClient, baseUri, "v1/auth/register", new AuthCredentials(email, password), cancellationToken);
        return await response.Content.ReadFromJsonAsync<RegistrationResponse>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty registration response");
    }

    public static async Task<TokenPair> LoginAsync(HttpClient httpClient, Uri baseUri, string email, string password, CancellationToken cancellationToken)
    {
        ValidateCredentials(email, password);
        using var response = await SendPublicAsync(httpClient, baseUri, "v1/auth/login", new AuthCredentials(email, password), cancellationToken);
        return await response.Content.ReadFromJsonAsync<TokenPair>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty token response");
    }

    public async Task<DeviceResponse> RegisterDeviceAsync(RegisterDeviceRequest request, CancellationToken cancellationToken)
    {
        using var response = await SendAsync(HttpMethod.Post, "v1/devices", request, null, cancellationToken);
        return await response.Content.ReadFromJsonAsync<DeviceResponse>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty device response");
    }

    public async Task<IReadOnlyList<DeviceResponse>> ListDevicesAsync(CancellationToken cancellationToken)
    {
        using var response = await SendAsync<object>(HttpMethod.Get, "v1/devices", null, null, cancellationToken);
        return await response.Content.ReadFromJsonAsync<List<DeviceResponse>>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty device list");
    }

    public async Task<IReadOnlyList<RegionResponse>> ListRegionsAsync(CancellationToken cancellationToken)
    {
        using var response = await SendAsync<object>(HttpMethod.Get, "v1/regions", null, null, cancellationToken);
        return await response.Content.ReadFromJsonAsync<List<RegionResponse>>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty region list");
    }

    public async Task<TunnelConfiguration> RequestConfigurationAsync(Guid deviceId, string regionId, string idempotencyKey, CancellationToken cancellationToken)
    {
        if (deviceId == Guid.Empty) throw new ArgumentException("Device ID is required", nameof(deviceId));
        if (string.IsNullOrWhiteSpace(regionId)) throw new ArgumentException("Region ID is required", nameof(regionId));
        if (string.IsNullOrWhiteSpace(idempotencyKey) || idempotencyKey.Length is < 16 or > 128)
            throw new ArgumentException("Invalid idempotency key", nameof(idempotencyKey));
        using var response = await SendAsync(HttpMethod.Post, $"v1/devices/{deviceId:D}/configuration", new ConfigurationRequest(regionId), idempotencyKey, cancellationToken);
        return await response.Content.ReadFromJsonAsync<TunnelConfiguration>(cancellationToken: cancellationToken)
            ?? throw new InvalidOperationException("API returned an empty configuration");
    }

    private async Task<HttpResponseMessage> SendAsync<T>(HttpMethod method, string path, T? body, string? idempotencyKey, CancellationToken cancellationToken)
    {
        using var request = new HttpRequestMessage(method, new Uri(_baseUri, path));
        if (body is not null) request.Content = JsonContent.Create(body);
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", _accessToken);
        if (idempotencyKey is not null) request.Headers.Add("Idempotency-Key", idempotencyKey);
        var response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        if (response.RequestMessage?.RequestUri != request.RequestUri ||
            response.RequestMessage?.RequestUri?.Scheme != Uri.UriSchemeHttps)
        {
            response.Dispose();
            throw new HttpRequestException("API redirect rejected");
        }
        if (!response.IsSuccessStatusCode) { response.Dispose(); throw new HttpRequestException($"API request failed: {(int)response.StatusCode}"); }
        return response;
    }

    private static Uri RequireHttps(Uri value)
    {
        if (!value.IsAbsoluteUri || value.Scheme != Uri.UriSchemeHttps || string.IsNullOrWhiteSpace(value.Host) ||
            !string.IsNullOrEmpty(value.UserInfo) || !string.IsNullOrEmpty(value.Query) || !string.IsNullOrEmpty(value.Fragment))
            throw new ArgumentException("API must use an HTTPS origin without embedded credentials", nameof(value));
        return new Uri(value.AbsoluteUri.TrimEnd('/') + "/", UriKind.Absolute);
    }

    private static string RequireToken(string value) =>
        string.IsNullOrWhiteSpace(value) ? throw new ArgumentException("API access token is required", nameof(value)) : value;

    private static void ValidateCredentials(string email, string password)
    {
        if (string.IsNullOrWhiteSpace(email) || email.Length > 320 || !email.Contains('@')) throw new ArgumentException("Invalid email", nameof(email));
        if (password.Length is < 12 or > 1024) throw new ArgumentException("Invalid password length", nameof(password));
    }

    private static async Task<HttpResponseMessage> SendPublicAsync<T>(HttpClient client, Uri baseUri, string path, T body, CancellationToken cancellationToken)
    {
        var root = RequireHttps(baseUri);
        using var request = new HttpRequestMessage(HttpMethod.Post, new Uri(root, path)) { Content = JsonContent.Create(body) };
        var response = await client.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        if (response.RequestMessage?.RequestUri != request.RequestUri || response.RequestMessage?.RequestUri?.Scheme != Uri.UriSchemeHttps)
        { response.Dispose(); throw new HttpRequestException("API redirect rejected"); }
        if (!response.IsSuccessStatusCode) { response.Dispose(); throw new HttpRequestException($"API request failed: {(int)response.StatusCode}"); }
        return response;
    }
}

public sealed record TunnelConfiguration(
    [property: JsonPropertyName("assignmentId")] string AssignmentId,
    [property: JsonPropertyName("endpoint")] string Endpoint,
    [property: JsonPropertyName("serverPublicKey")] string ServerPublicKey,
    [property: JsonPropertyName("addresses")] IReadOnlyList<string> Addresses,
    [property: JsonPropertyName("dnsServers")] IReadOnlyList<string> DnsServers,
    [property: JsonPropertyName("allowedIps")] IReadOnlyList<string> AllowedIps,
    [property: JsonPropertyName("persistentKeepaliveSeconds")] int PersistentKeepaliveSeconds);
