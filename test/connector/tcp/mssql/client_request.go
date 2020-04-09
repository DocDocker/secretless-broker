package mssqltest

import (
	"net"
	"strconv"

	"github.com/cyberark/secretless-broker/internal/log"
	"github.com/cyberark/secretless-broker/internal/plugin/connectors/tcp"
	"github.com/cyberark/secretless-broker/internal/plugin/connectors/tcp/mssql"
	"github.com/cyberark/secretless-broker/pkg/secretless/plugin/connector"
)

type clientRequest struct {
	params dbConfigParams
	query  string
}

type clientResponse struct {
	out string
	err error
}

func cloneCredentials(original map[string][]byte) map[string][]byte {
	clone := make(map[string][]byte)
	for key, value := range original {
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)

		clone[key] = valueCopy
	}
	return clone
}

func (clientReq clientRequest) proxyRequest(
	executor dbQueryExecutor,
	credentials map[string][]byte,
) (string, string, error) {

	// Copy credentials. This prevents any mutation or zeroization
	credentials = cloneCredentials(credentials)

	var err error

	logger := log.New(true)
	testLogger := logger.CopyWith("[TEST]", true)

	// Create proxy service listener
	proxySvcListener, err := createListenerOnPort("0")
	if err != nil {
		return "", "", err
	}
	secretlessSvcHost, secretlessSvcPort, err := net.SplitHostPort(
		proxySvcListener.Addr().String(),
	)
	if err != nil {
		return "", "", err
	}
	secretlessSvcPortInt, err := strconv.Atoi(secretlessSvcPort)
	if err != nil {
		return "", "", err
	}

	// Ensure the proxy service listener is closed on return
	defer func() {
		_ = proxySvcListener.Close()
	}()

	// Create the proxy service
	proxySvc, err := tcp.NewProxyService(
		mssql.NewConnector(
			connector.NewResources(nil, logger),
		),
		proxySvcListener,
		logger,
		func() (bytes map[string][]byte, e error) {
			return credentials, nil
		},
	)
	if err != nil {
		return "", "", err
	}
	// Ensure the proxy service is stopped on return
	defer func() {
		_ = proxySvc.Stop()
	}()

	// Start the proxy service
	go func() {
		testLogger.Info("Start proxy service")
		_ = proxySvc.Start()
	}()

	// Make the client request via the proxy service
	clientResChan := make(chan clientResponse)
	go func() {
		testLogger.Info("Make client request to proxy at", secretlessSvcHost, secretlessSvcPort)

		out, err := executor(
			dbConfig{
				Host:           secretlessSvcHost,
				Port:           secretlessSvcPortInt,
				Username:       "dummy",
				Password:       "dummy",
				dbConfigParams: clientReq.params,
			},
			clientReq.query,
		)

		testLogger.Info("Received client response")

		if clientResChan == nil {
			return
		}

		clientResChan <- clientResponse{
			out: out,
			err: err,
		}
	}()

	res := <-clientResChan
	return res.out, secretlessSvcPort, res.err
}

// NOTE: proxyToCreatedMock proxies the request to a mock server that terminates the request after the handshake
// This can have unintended effects. gomssql in particular does some weird retry, when a query is prepared!
// TODO: find out this weird gomssqldb behavior.
func (clientReq clientRequest) proxyToCreatedMock(
	executor dbQueryExecutor,
	credentials map[string][]byte,
) (*mockTargetCapture, string, error) {
	// Create mock target
	mt, err := newMockTarget("0")
	if err != nil {
		return nil, "", err
	}
	defer func() {
		_ = mt.close()
	}()

	// Gather credentials
	baseCredentials := map[string][]byte{
		"host": []byte(mt.host),
		"port": []byte(mt.port),
	}
	for key, value := range credentials {
		baseCredentials[key] = value
	}

	// Accept on mock target
	mtResChan := mt.accept()

	// We don't expect anything useful to come back from the client request.
	// This is a fire and forget
	_, secretlessPort, _ := clientReq.proxyRequest(
		executor,
		baseCredentials,
	)

	mtRes := <-mtResChan

	return mtRes.capture, secretlessPort, mtRes.err
}

func (clientReq clientRequest) proxyToMock(
	executor dbQueryExecutor,
	credentials map[string][]byte,
	mt *mockTarget,
) (*mockTargetCapture, string, error) {
	// Gather credentials
	baseCredentials := map[string][]byte{
		"host": []byte(mt.host),
		"port": []byte(mt.port),
	}
	for key, value := range credentials {
		baseCredentials[key] = value
	}

	// Accept on mock target
	mtResChan := mt.accept()

	// We don't expect anything useful to come back from the client request.
	// This is a fire and forget
	_, secretlessPort, _ := clientReq.proxyRequest(
		executor,
		baseCredentials,
	)

	mtRes := <-mtResChan

	return mtRes.capture, secretlessPort, mtRes.err
}
