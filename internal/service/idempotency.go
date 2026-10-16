package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
)

var requestKeyPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func ValidRequestKey(key string) bool { return requestKeyPattern.MatchString(key) }
func creationKey(keys []string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	if len(keys) != 1 || !ValidRequestKey(keys[0]) {
		return "", Invalid("Idempotency-Key must be a UUID")
	}
	return keys[0], nil
}
func cachedCreation(ctx context.Context, r Repository, actor int64, key, operation string, input, output any) (bool, error) {
	if key == "" {
		return false, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(data)
	cached, err := r.Creation(ctx, actor, key, operation, hex.EncodeToString(sum[:]))
	if err != nil {
		return false, err
	}
	if len(cached) == 0 {
		return false, nil
	}
	return true, json.Unmarshal(cached, output)
}
func saveCreation(ctx context.Context, r Repository, actor int64, key string, output any) error {
	if key == "" {
		return nil
	}
	data, err := json.Marshal(output)
	if err != nil {
		return err
	}
	return r.SaveCreation(ctx, actor, key, data)
}
