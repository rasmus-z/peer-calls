package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

func ReadFile(filename string, c *Config) (err error) {
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("Error opening YAML file: %w", err)
	}
	err = ReadYAML(f, c)
	f.Close()
	return err
}

func ReadFiles(filenames []string, c *Config) (err error) {
	for _, filename := range filenames {
		err = ReadFile(filename, c)
		if err != nil {
			break
		}
	}
	return err
}

func Init(c *Config) {
	c.BindPort = 3000
	c.Network.Type = NetworkTypeMesh
	c.Store.Type = StoreTypeMemory
	c.ICEServers = []ICEServer{{
		URLs: []string{"stun:stun.l.google.com:19302"},
	}, {
		URLs: []string{"stun:global.stun.twilio.com:3478?transport=udp"},
	}}
}

func Read(filenames []string) (c Config, err error) {
	Init(&c)
	err = ReadFiles(filenames, &c)
	ReadEnv("PEERCALLS_", &c)
	return c, err
}

func ReadYAML(reader io.Reader, c *Config) error {
	decoder := yaml.NewDecoder(reader)
	if err := decoder.Decode(c); err != nil {
		return fmt.Errorf("Error parsing YAML: %w", err)
	}
	return nil
}

func ReadEnv(prefix string, c *Config) {
	setEnvString(&c.BaseURL, prefix+"BASE_URL")
	setEnvString(&c.BindHost, prefix+"BIND_HOST")
	setEnvInt(&c.BindPort, prefix+"BIND_PORT")
	setEnvString(&c.TLS.Cert, prefix+"TLS_CERT")
	setEnvString(&c.TLS.Key, prefix+"TLS_KEY")

	setEnvStoreType(&c.Store.Type, prefix+"STORE_TYPE")
	setEnvString(&c.Store.Redis.Host, prefix+"STORE_REDIS_HOST")
	setEnvInt(&c.Store.Redis.Port, prefix+"STORE_REDIS_PORT")
	setEnvString(&c.Store.Redis.Prefix, prefix+"STORE_REDIS_PREFIX")

	setEnvNetworkType(&c.Network.Type, prefix+"NETWORK_TYPE")
	setEnvStringArray(&c.Network.SFU.Interfaces, prefix+"NETWORK_SFU_INTERFACES")

	var ice ICEServer
	setEnvSlice(&ice.URLs, prefix+"ICE_SERVER_URLS")
	if len(ice.URLs) > 0 {
		setEnvAuthType(&ice.AuthType, prefix+"ICE_SERVER_AUTH_TYPE")
		setEnvString(&ice.AuthSecret.Secret, prefix+"ICE_SERVER_SECRET")
		setEnvString(&ice.AuthSecret.Username, prefix+"ICE_SERVER_USERNAME")
		c.ICEServers = append(c.ICEServers, ice)
	}
}

func setEnvSlice(dest *[]string, name string) {
	value := os.Getenv(name)
	for _, v := range strings.Split(value, ",") {
		if v != "" {
			*dest = append(*dest, v)
		}
	}
}

func setEnvString(dest *string, name string) {
	value := os.Getenv(name)
	if value != "" {
		*dest = value
	}
}

func setEnvInt(dest *int, name string) {
	value, err := strconv.Atoi(os.Getenv(name))
	if err == nil {
		*dest = value
	}
}

func setEnvAuthType(authType *AuthType, name string) {
	value := os.Getenv(name)
	switch AuthType(value) {
	case AuthTypeSecret:
		*authType = AuthTypeSecret
	case AuthTypeNone:
		*authType = AuthTypeNone
	}
}

func setEnvNetworkType(networkType *NetworkType, name string) {
	value := os.Getenv(name)
	switch NetworkType(value) {
	case NetworkTypeMesh:
		*networkType = NetworkTypeMesh
	case NetworkTypeSFU:
		*networkType = NetworkTypeSFU
	}
}

func setEnvStoreType(storeType *StoreType, name string) {
	value := os.Getenv(name)
	switch StoreType(value) {
	case StoreTypeRedis:
		*storeType = StoreTypeRedis
	case StoreTypeMemory:
		*storeType = StoreTypeMemory
	}
}

func setEnvStringArray(interfaces *[]string, name string) {
	value := os.Getenv(name)
	if value != "" {
		values := strings.Split(value, ",")
		*interfaces = values
	}
}
