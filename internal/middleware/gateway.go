package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/status"

	"github.com/kurnhyalcantara/temtem/internal/constants"
)

// GatewayOptions returns the ServeMux options shared by all features:
// a JSON error body shaped as {"code","message"} and propagation of the
// authorization and x-request-id headers into gRPC metadata.
func GatewayOptions() []runtime.ServeMuxOption {
	return []runtime.ServeMuxOption{
		runtime.WithErrorHandler(errorHandler),
		runtime.WithIncomingHeaderMatcher(headerMatcher),
	}
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func errorHandler(ctx context.Context, _ *runtime.ServeMux, _ runtime.Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	st := status.Convert(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(runtime.HTTPStatusFromCode(st.Code()))
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:    st.Code().String(),
		Message: st.Message(),
	})
}

func headerMatcher(key string) (string, bool) {
	switch strings.ToLower(key) {
	case constants.HeaderAuthorization, constants.HeaderRequestID:
		return strings.ToLower(key), true
	default:
		return runtime.DefaultHeaderMatcher(key)
	}
}
