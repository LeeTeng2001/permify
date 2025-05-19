package permify

import (
	"context"

	"github.com/Permify/permify/internal/config"
	"github.com/Permify/permify/internal/engines"
	"github.com/Permify/permify/internal/factories"
	"github.com/Permify/permify/internal/invoke"
	"github.com/Permify/permify/internal/storage"
	"github.com/Permify/permify/pkg/database"
	"github.com/Permify/permify/pkg/dsl/compiler"
	"github.com/Permify/permify/pkg/dsl/parser"
	base "github.com/Permify/permify/pkg/pb/base/v1"
	"github.com/Permify/permify/pkg/token"
	"github.com/Permify/permify/pkg/tuple"
	"github.com/rs/xid"
)

const DefaultTenantID = "default"

var DefaultSnapToken = token.NewNoopToken().Encode().String()

type Engine struct {
	invoker   *invoke.DirectInvoker
	entityDef []*base.EntityDefinition
	ruleDef   []*base.RuleDefinition
}

func NewEngine(ctx context.Context, schema string, relationship []string) (*Engine, error) {
	db, err := factories.DatabaseFactory(
		config.Database{
			Engine: "memory",
		},
	)
	if err != nil {
		return nil, err
	}

	// parse schema definition
	sch, err := parser.NewParser(schema).Parse()
	if err != nil {
		return nil, err
	}
	entityDef, ruleDef, err := compiler.NewCompiler(false, sch).Compile()
	if err != nil {
		return nil, err
	}
	version := xid.New().String()
	cnf := make([]storage.SchemaDefinition, 0, len(sch.Statements))
	for _, st := range sch.Statements {
		cnf = append(cnf, storage.SchemaDefinition{
			TenantID:             DefaultTenantID,
			Version:              version,
			Name:                 st.GetName(),
			SerializedDefinition: []byte(st.String()),
		})
	}
	if err != nil {
		return nil, err
	}

	// write to memory db
	schemaWriter := factories.SchemaWriterFactory(db)
	if err := schemaWriter.WriteSchema(ctx, cnf); err != nil {
		return nil, err
	}

	// rw
	schemaReader := factories.SchemaReaderFactory(db)
	dataReader := factories.DataReaderFactory(db)
	dataWriter := factories.DataWriterFactory(db)
	checkEngine := engines.NewCheckEngine(schemaReader, dataReader)

	invoker := invoke.NewDirectInvoker(
		schemaReader,
		dataReader,
		checkEngine,
		nil,
		nil,
		nil,
	)
	checkEngine.SetInvoker(invoker)

	// check relationship definitions
	var tuples []*base.Tuple
	for _, relationship := range relationship {
		tup, err := tuple.Tuple(relationship)
		if err != nil {
			return nil, err
		}
		tuples = append(tuples, tup)
	}

	_, err = dataWriter.Write(ctx, DefaultTenantID, database.NewTupleCollection(tuples...), database.NewAttributeCollection())
	if err != nil {
		return nil, err
	}

	return &Engine{
		invoker:   invoker,
		entityDef: entityDef,
		ruleDef:   ruleDef,
	}, nil
}

func (e *Engine) Check(ctx context.Context, subject, action, entity string) (bool, error) {
	entityObj, err := tuple.E(entity)
	if err != nil {
		return false, err
	}

	ear, err := tuple.EAR(subject)
	if err != nil {
		return false, err
	}

	subjectObj := &base.Subject{
		Type:     ear.GetEntity().GetType(),
		Id:       ear.GetEntity().GetId(),
		Relation: ear.GetRelation(),
	}

	response, err := e.invoker.Check(ctx, &base.PermissionCheckRequest{
		TenantId:   DefaultTenantID,
		Entity:     entityObj,
		Subject:    subjectObj,
		Permission: action,
		Metadata: &base.PermissionCheckRequestMetadata{
			SnapToken:     DefaultSnapToken,
			SchemaVersion: "",
			Depth:         20,
		},
	})
	if err != nil {
		return false, err
	}

	return response.GetCan() == base.CheckResult_CHECK_RESULT_ALLOWED, nil
}

func (e *Engine) GetEntityDefinition() []*base.EntityDefinition {
	return e.entityDef
}

func (e *Engine) GetRuleDefinition() []*base.RuleDefinition {
	return e.ruleDef
}
