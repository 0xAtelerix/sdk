package gosdk

import emitterproto "github.com/0xAtelerix/sdk/proto"

// todo our implementation of health server
type HealthServer struct {
	emitterproto.UnimplementedHealthServer
}
