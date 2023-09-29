package fakeprover

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-resty/resty/v2"
	"github.com/labstack/gommon/log"
	"github.com/taikoxyz/taiko-client/bindings"
	"github.com/taikoxyz/taiko-client/pkg/rpc"
	capacity "github.com/taikoxyz/taiko-client/prover/capacity_manager"
	"github.com/taikoxyz/taiko-client/prover/server"
	"github.com/taikoxyz/taiko-client/testutils"
)

// New starts a new prover server that has channel listeners to respond and react
// to requests for capacity, which provers can call.
func New(
	protocolConfig *bindings.TaikoDataConfig,
	jwtSecret []byte,
	rpcClient *rpc.Client,
	proverPrivKey *ecdsa.PrivateKey,
	capacityManager *capacity.CapacityManager,
	url *url.URL,
) (*server.ProverServer, error) {
	srv, err := server.New(&server.NewProverServerOpts{
		ProverPrivateKey: proverPrivKey,
		MinProofFee:      common.Big1,
		MaxExpiry:        24 * time.Hour,
		CapacityManager:  capacityManager,
		TaikoL1Address:   testutils.TaikoL1Address,
		Rpc:              rpcClient,
		Bond:             protocolConfig.ProofBond,
		IsOracle:         true,
	})
	if err != nil {
		return nil, err
	}

	go func() {
		if err := srv.Start(fmt.Sprintf(":%v", url.Port())); !errors.Is(err, http.ErrServerClosed) {
			log.Error("Failed to start prover server", "error", err)
		}
	}()

	// Wait till the server fully started.
	if err := backoff.Retry(func() error {
		res, err := resty.New().R().Get(url.String() + "/healthz")
		if err != nil {
			return err
		}
		if !res.IsSuccess() {
			return fmt.Errorf("invalid response status code: %d", res.StatusCode())
		}

		return nil
	}, backoff.NewExponentialBackOff()); err != nil {
		return nil, err
	}
	return srv, nil
}
