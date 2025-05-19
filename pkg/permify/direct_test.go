package permify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

const testSchema = `
entity user {} 

entity organization {
    relation guest @user
    relation test @user
    relation qa @user
    relation user_ops @user
    relation ops @user
    relation sre @user

    permission default_ops_permission = ops or sre
    permission default_user_ops_permission = default_ops_permission or user_ops
    permission default_qa_permission = default_user_ops_permission or qa
    permission default_test_permission = default_qa_permission or test
    permission default_guest_permission = default_test_permission or guest
}

entity DefaultResource {
    relation org @organization

    action edit = org.default_ops_permission
    action read = org.default_guest_permission
}
`

func TestDirectUsage(t *testing.T) {
	// test assertions
	type check struct {
		entity     string
		subject    string
		assertions map[string]bool
	}
	tests := struct {
		relationships []string
		checks        []check
	}{
		relationships: []string{
			"organization:mhy#sre@user:bob",
			"organization:mhy#guest@user:userguest",
			"DefaultResource:hc#org@organization:mhy",
		},
		checks: []check{
			{
				entity:  "DefaultResource:hc",
				subject: "user:bob",
				assertions: map[string]bool{
					"edit": true,
					"read": true,
				},
			},
			{
				entity:  "DefaultResource:hc",
				subject: "user:userguest",
				assertions: map[string]bool{
					"edit": false,
					"read": true,
				},
			},
			{
				entity:  "DefaultResource:hc",
				subject: "user:notexist",
				assertions: map[string]bool{
					"edit": false,
					"read": false,
				},
			},
		},
	}

	// create engine
	engine, err := NewEngine(context.Background(), testSchema, tests.relationships)
	assert.NoError(t, err)

	for _, check := range tests.checks {
		for permission, res := range check.assertions {
			allowed, err := engine.Check(context.Background(), check.subject, permission, check.entity)
			assert.NoError(t, err)
			assert.Equal(t, res, allowed)
		}
	}
}
