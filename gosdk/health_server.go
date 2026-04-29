package gosdk

import emitterproto "github.com/0xAtelerix/sdk/gosdk/proto"

// HealthServer is our implementation of the emitter health server.
type HealthServer struct {
	emitterproto.UnimplementedHealthServer
}
