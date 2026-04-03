package storage

import (
	"fmt"
	"os"
	"strings"
)

// ReadAlibabaCloudCredentialsFromINI reads AccessKey pair from an INI file.
// Expected section [credentials] with keys:
//   - alibaba_cloud_access_key_id
//   - alibaba_cloud_access_key_secret
func ReadAlibabaCloudCredentialsFromINI(path string) (accessKeyID, secretAccessKey string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read credentials ini: %w", err)
	}
	var inCredentials bool
	var idSet, secretSet bool
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			inCredentials = section == "credentials"
			continue
		}
		if !inCredentials {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k := strings.TrimSpace(strings.ToLower(key))
		v := strings.TrimSpace(val)
		switch k {
		case "alibaba_cloud_access_key_id":
			accessKeyID = v
			idSet = true
		case "alibaba_cloud_access_key_secret":
			secretAccessKey = v
			secretSet = true
		}
	}
	if !idSet || accessKeyID == "" {
		return "", "", fmt.Errorf("credentials ini: missing alibaba_cloud_access_key_id in [credentials]")
	}
	if !secretSet || secretAccessKey == "" {
		return "", "", fmt.Errorf("credentials ini: missing alibaba_cloud_access_key_secret in [credentials]")
	}
	return accessKeyID, secretAccessKey, nil
}
