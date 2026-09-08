package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonVersion uint32 = 19
	argonTime           = 3
	argonMemory         = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen         = 32
	argonSaltLen        = 16
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

// HashPassword returns a PHC-formatted Argon2id hash. The parameters are
// deliberately explicit so upgrades can be detected and rehashes planned.
func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password must contain 12 to 1024 bytes")
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	encode := base64.RawStdEncoding.EncodeToString
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, argonMemory, argonTime, argonThreads, encode(salt), encode(key)), nil
}

func VerifyPassword(password, encoded string) bool {
	version, memory, iterations, threads, salt, expected, ok := parsePHC(encoded)
	if !ok || version != argonVersion || len(salt) < 16 || len(expected) != argonKeyLen {
		return false
	}
	if memory < 16*1024 || memory > 2*1024*1024 || iterations == 0 || iterations > 10 || threads == 0 || threads > 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func parsePHC(encoded string) (version uint32, memory, iterations uint32, threads uint8, salt, key []byte, ok bool) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || !strings.HasPrefix(parts[2], "v=") {
		return 0, 0, 0, 0, nil, nil, false
	}
	var err error
	var parsed uint64
	if parsed, err = strconv.ParseUint(strings.TrimPrefix(parts[2], "v="), 10, 32); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	version = uint32(parsed)
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return 0, 0, 0, 0, nil, nil, false
	}
	values := make(map[string]string, len(params))
	for _, parameter := range params {
		pair := strings.SplitN(parameter, "=", 2)
		if len(pair) != 2 || (pair[0] != "m" && pair[0] != "t" && pair[0] != "p") {
			return 0, 0, 0, 0, nil, nil, false
		}
		if _, exists := values[pair[0]]; exists {
			return 0, 0, 0, 0, nil, nil, false
		}
		values[pair[0]] = pair[1]
	}
	if parsed, err = strconv.ParseUint(values["m"], 10, 32); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	memory = uint32(parsed)
	if parsed, err = strconv.ParseUint(values["t"], 10, 32); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	iterations = uint32(parsed)
	if parsed, err = strconv.ParseUint(values["p"], 10, 8); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	threads = uint8(parsed)
	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil {
		return 0, 0, 0, 0, nil, nil, false
	}
	return version, memory, iterations, threads, salt, key, true
}
