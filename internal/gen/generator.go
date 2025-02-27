package gen

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/xerrors"

	"github.com/gotd/getdoc"
	"github.com/gotd/tl"
)

func definitionType(d tl.Definition) string {
	if len(d.Namespace) == 0 {
		return d.Name
	}
	return fmt.Sprintf("%s.%s", strings.Join(d.Namespace, "."), d.Name)
}

// Generator generates go types from tl.Schema.
type Generator struct {
	schema *tl.Schema

	// classes type bindings, key is TL type.
	classes map[string]classBinding
	// types bindings, key is TL type.
	types map[string]typeBinding

	// structs definitions.
	structs []structDef
	// interfaces definitions.
	interfaces []interfaceDef
	// errorChecks definitions.
	errorChecks []errCheckDef

	// constructor mappings.
	mappings map[string][]constructorMapping

	// registry of type ids.
	registry []bindingDef

	// docBase is base url for documentation.
	docBase      *url.URL
	doc          *getdoc.Doc
	docLineLimit int

	generateServer bool
}

type generateOptions struct {
	docBaseURL     string
	generateServer bool
}

// Option that configures generation.
type Option func(o *generateOptions)

// WithServer enables experimental server generation.
func WithServer() Option {
	return func(o *generateOptions) {
		o.generateServer = true
	}
}

// WithDocumentation will embed documentation references to generated code.
//
// If base is https://core.telegram.org, documentation content will be also
// embedded.
func WithDocumentation(base string) Option {
	return func(o *generateOptions) {
		o.docBaseURL = base
	}
}

// NewGenerator initializes and returns new Generator from tl.Schema.
func NewGenerator(s *tl.Schema, options ...Option) (*Generator, error) {
	genOpt := &generateOptions{}
	for _, opt := range options {
		opt(genOpt)
	}
	g := &Generator{
		schema:         s,
		classes:        map[string]classBinding{},
		types:          map[string]typeBinding{},
		mappings:       map[string][]constructorMapping{},
		docLineLimit:   87,
		generateServer: genOpt.generateServer,
	}
	if genOpt.docBaseURL != "" {
		u, err := url.Parse(genOpt.docBaseURL)
		if err != nil {
			return nil, xerrors.Errorf("failed to parse docBase: %w", err)
		}
		g.docBase = u

		if u.Host == "core.telegram.org" {
			// Using embedded documentation.
			// TODO(ernado): Get actual layer
			doc, err := getdoc.Load(getdoc.LayerLatest)
			if err != nil {
				return nil, xerrors.Errorf("failed to get documentation: %w", err)
			}
			g.doc = doc
		}
	}
	if err := g.makeBindings(); err != nil {
		return nil, xerrors.Errorf("failed to make type bindings: %w", err)
	}
	if err := g.makeStructures(); err != nil {
		return nil, xerrors.Errorf("failed to generate go structures: %w", err)
	}
	g.makeInterfaces()
	g.makeErrors()

	return g, nil
}
