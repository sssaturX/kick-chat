package refreshstore

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix         = "refresh:"
	licenseSetPrefix  = "license_refresh:"
	defaultExpiration = 7 * 24 * time.Hour
)

// Entry stored in Redis for a refresh token.
type Entry struct {
	LicenseID string `json:"license_id"`
	DeviceID  string `json:"device_id"`
}

func licenseSetKey(licID string) string { return licenseSetPrefix + licID }

// Store saves a refresh token mapping to license_id and device_id. Token is the value to use in refresh requests.
func Store(ctx context.Context, rdb *redis.Client, token string, licenseID, deviceID uuid.UUID, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = defaultExpiration
	}
	entry := Entry{LicenseID: licenseID.String(), DeviceID: deviceID.String()}
	val, err := json.Marshal(&entry)
	if err != nil {
		return err
	}
	pipe := rdb.Pipeline()
	pipe.Set(ctx, keyPrefix+token, val, ttl)
	pipe.SAdd(ctx, licenseSetKey(licenseID.String()), token)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	// Set TTL on the set so it doesn't leak if we never revoke
	rdb.Expire(ctx, licenseSetKey(licenseID.String()), ttl+24*time.Hour)
	return nil
}

// Get retrieves license_id and device_id for a refresh token. Returns nil if not found.
func Get(ctx context.Context, rdb *redis.Client, token string) (*Entry, error) {
	val, err := rdb.Get(ctx, keyPrefix+token).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var e Entry
	if err := json.Unmarshal(val, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

// Delete removes one refresh token (e.g. after issuing new one on refresh).
func Delete(ctx context.Context, rdb *redis.Client, token string) error {
	val, err := rdb.Get(ctx, keyPrefix+token).Bytes()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	var e Entry
	if err := json.Unmarshal(val, &e); err != nil {
		return err
	}
	pipe := rdb.Pipeline()
	pipe.Del(ctx, keyPrefix+token)
	pipe.SRem(ctx, licenseSetKey(e.LicenseID), token)
	_, err = pipe.Exec(ctx)
	return err
}

// RevokeLicense deletes all refresh tokens for the given license (on admin revoke).
func RevokeLicense(ctx context.Context, rdb *redis.Client, licenseID uuid.UUID) error {
	licStr := licenseID.String()
	tokens, err := rdb.SMembers(ctx, licenseSetKey(licStr)).Result()
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil
	}
	pipe := rdb.Pipeline()
	for _, t := range tokens {
		pipe.Del(ctx, keyPrefix+t)
	}
	pipe.Del(ctx, licenseSetKey(licStr))
	_, err = pipe.Exec(ctx)
	return err
}

// Token returns a new random token string.
func Token() string {
	return uuid.New().String() + "-" + uuid.New().String()
}
