module github.com/stevendborrelli/function-ordering

go 1.26.7

require (
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.11
)

require github.com/crossplane/crossplane/v2 v2.4.0

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

// The dependencies field this function returns does not exist in any released
// Crossplane. It comes from the composed resource ordering prototype, so the
// proto package has to be taken from that branch.
//
// Drop this once the field lands upstream, and require a real version instead.
replace github.com/crossplane/crossplane/v2 => github.com/stevendborrelli/crossplane/v2 v2.0.0-20260917093218-28de049245f7
