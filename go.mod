module github.com/gitpod-io/gitpod/run-gp

go 1.18

require (
	github.com/Masterminds/semver v1.5.0
	github.com/gitpod-io/gitpod/gitpod-protocol v0.0.0-00010101000000-000000000000
	github.com/google/go-github/v45 v45.1.0
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf
	github.com/mattn/go-isatty v0.0.14
	github.com/pterm/pterm v0.12.41
	github.com/segmentio/analytics-go/v3 v3.2.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/vmware-labs/yaml-jsonpath v0.3.2
	golang.org/x/oauth2 v0.0.0-20220608161450-d0670ef3b1eb
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/atomicgo/cursor v0.0.1 // indirect
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/dprotaso/go-yit v0.0.0-20191028211022-135eb7262960 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.4.2 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gookit/color v1.5.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/segmentio/backo-go v1.0.0 // indirect
	github.com/sourcegraph/jsonrpc2 v0.0.0-20200429184054-15c2290dcb37 // indirect
	github.com/xo/terminfo v0.0.0-20210125001918-ca9a967f8778 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sys v0.0.0-20220319134239-a9b59b0215f8 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)

replace github.com/gitpod-io/gitpod/gitpod-protocol => github.com/gitpod-io/gitpod/components/gitpod-protocol/go v0.0.0-20220615132424-21a462d793da
