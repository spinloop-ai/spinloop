package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// gatewayTimeout bounds one topology read: the gateway's own worst case is a
// single fan-out over its fleet, so this is room for the read, not a second
// opinion on the fleet.
var gatewayTimeout = 30 * time.Second

// GatewayTopologist is the Topologist the command runs on: the fleet's
// gateway, read at its topology endpoint. The gateway is the orchestrator's
// only view of the fleet, so an error from Topology is the gateway not
// answering, and the run's contract ends on it, naming the gateway.
type GatewayTopologist struct {
	// Gateway is the gateway's address, as the command was given it.
	Gateway string
	// Token is the gateway's bearer token, resolved by the caller.
	Token string
	// Client issues the reads; a variable so a test stands in.
	Client *http.Client
}

// NewGatewayTopologist builds the topologist pointed at the gateway, holding
// the token it presents.
func NewGatewayTopologist(gateway, token string) *GatewayTopologist {
	return &GatewayTopologist{
		Gateway: gateway,
		Token:   token,
		Client:  &http.Client{Timeout: gatewayTimeout},
	}
}

// Topology reads the fleet's current topology from the gateway.
func (g *GatewayTopologist) Topology(ctx context.Context) (Topology, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Gateway+"/v1/fleet", nil)
	if err != nil {
		return Topology{}, fmt.Errorf("the gateway %s is not an address: %v", g.Gateway, err)
	}
	req.Header.Set("Authorization", "Bearer "+g.Token)
	resp, err := g.Client.Do(req)
	if err != nil {
		return Topology{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Topology{}, fmt.Errorf("the gateway %s answered %s", g.Gateway, resp.Status)
	}
	var topo Topology
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&topo); err != nil {
		return Topology{}, fmt.Errorf("the gateway %s's topology is not a record: %v", g.Gateway, err)
	}
	return topo, nil
}
