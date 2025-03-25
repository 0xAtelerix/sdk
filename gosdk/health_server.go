package gosdk

import emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"

// todo our implementation of health server
type HealthServer struct {
	emitterproto.UnimplementedHealthServer
}
