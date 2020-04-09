package mssqltest

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cyberark/secretless-broker/test/util/testutil"
)

var envCfg = testutil.NewDbConfigFromEnv()

func TestRealE2E(t *testing.T) {
	clientRequest := clientRequest{
		params: dbConfigParams{
			Database: "tempdb",
			ReadOnly: false,
		},
		query: "",
	}

	_, _, err := clientRequest.proxyRequest(
		sqlcmdExec,
		map[string][]byte{
			"sslmode":  []byte("disable"),
			"username": []byte(envCfg.User),
			"password": []byte(envCfg.Password),
			"host":     []byte(envCfg.HostWithTLS),
			"port":     []byte(envCfg.Port),
		},
	)
	if !assert.NoError(t, err) {
		return
	}
}

func TestMockE2E(t *testing.T) {
	clientRequest := clientRequest{
		params: dbConfigParams{
			Database: "meow",
			ReadOnly: false,
		},
		query: "",
	}
	_, _, err := clientRequest.proxyToCreatedMock(
		gomssqlExec,
		map[string][]byte{
			"sslmode": []byte("disable"),
		})

	assert.NoError(t, err)
}
