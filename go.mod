module github.com/gravitational/logging-app

go 1.15

require (
	github.com/LK4D4/joincontext v0.0.0-20171026170139-1724345da6d5
	github.com/alecthomas/participle v0.2.1
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gravitational/logrus v0.10.1-0.20180402202453-dcdb95d728db
	github.com/gravitational/trace v0.0.0-20190218181455-5d6afe38af2b
	github.com/jrivets/log4g v0.0.0-20191016233753-c02c5046dc98
	github.com/julienschmidt/httprouter v1.2.0
	github.com/logrange/logrange v0.1.46
	github.com/logrange/range v0.0.0-20191016234805-44ada6216b88
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/urfave/cli.v2 v2.0.0-20180128182452-d3ae77c26ac8
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
)

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
