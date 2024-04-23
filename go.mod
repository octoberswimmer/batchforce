module github.com/octoberswimmer/batchforce

go 1.21.4

toolchain go1.21.5

require (
	github.com/ForceCLI/force v1.0.5-0.20240214162012-1d63a1439c6a
	github.com/antlr4-go/antlr/v4 v4.13.0
	github.com/benhoyt/goawk v1.21.0
	github.com/clbanning/mxj v1.8.4
	github.com/expr-lang/expr v1.16.0
	github.com/grokify/html-strip-tags-go v0.0.1
	github.com/mitchellh/mapstructure v1.5.0
	github.com/octoberswimmer/apexfmt v0.0.0-20231227154000-947cf367deb7
	github.com/recursionpharma/go-csv-map v0.0.0-20160524001940-792523c65ae9
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/ForceCLI/config v0.0.0-20230217143549-9149d42a3c99 // indirect
	github.com/ForceCLI/inflect v0.0.0-20130829110746-cc00b5ad7a6a // indirect
	github.com/ViViDboarder/gotifier v0.0.0-20140619195515-0f19f3d7c54c // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/onsi/gomega v1.23.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/smartystreets/goconvey v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/exp v0.0.0-20231226003508-02704c960a9b // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Can be removed after https://github.com/expr-lang/expr/pull/633 is resolved
replace github.com/expr-lang/expr v1.16.0 => github.com/cwarden/expr v0.0.0-20240423160201-50d02973b2f2
