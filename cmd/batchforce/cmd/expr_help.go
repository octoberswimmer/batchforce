package cmd

import (
	_ "embed"
	// The following go:generate directive copies the Expr language definition
	// markdown into this package from the module cache.
)

//go:generate sh -c "cp \"$(go env GOMODCACHE)/github.com/expr-lang/expr@$(go list -m -f '{{.Version}}' github.com/expr-lang/expr)/docs/language-definition.md\" docs/language-definition.md"

// exprLangDef embeds the Expr language definition markdown.
//
//go:embed docs/language-definition.md
var exprLangDef string
