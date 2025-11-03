package permify

import (
	"context"
	"fmt"
	"testing"

	"github.com/samber/lo"
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

    permission test_empty
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
		relationships    []string
		directPermission map[string][]lo.Tuple2[string, string]
		checks           []check
	}{
		relationships: []string{
			"organization:mhy#sre@user:bob",
			"organization:mhy#guest@user:userguest",
			"DefaultResource:hc#org@organization:mhy",
		},
		directPermission: map[string][]lo.Tuple2[string, string]{
			"user:userguest": {
				lo.T2("DefaultResource:hc", "edit"),
			},
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
					"edit": true,
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

	for subject, entityPermissionList := range tests.directPermission {
		engine.UpdateDirectPermission(context.Background(), subject, entityPermissionList)
	}

	for _, check := range tests.checks {
		for permission, res := range check.assertions {
			allowed, err := engine.Check(context.Background(), check.subject, permission, check.entity)
			assert.NoError(t, err)
			assert.Equal(t, res, allowed, fmt.Sprintf("permission: %s, subject: %s, entity: %s", permission, check.subject, check.entity))
		}
	}
}

func TestUpdateDeleteRelationship(t *testing.T) {
	engine, err := NewEngine(context.Background(), testSchema, []string{
		"DefaultResource:hc#org@organization:mhy",
	})
	assert.NoError(t, err)

	// bob does not have permission (not registered)
	allowed, err := engine.Check(context.Background(), "user:bob", "edit", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.False(t, allowed)
	allowed, err = engine.Check(context.Background(), "user:bob", "read", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.False(t, allowed)

	// bob has guest permisison
	err = engine.UpdateRelationships(context.Background(), []string{
		"organization:mhy#guest@user:bob",
	})
	assert.NoError(t, err)
	allowed, err = engine.Check(context.Background(), "user:bob", "edit", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.False(t, allowed)
	allowed, err = engine.Check(context.Background(), "user:bob", "read", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// bob has admin permission
	err = engine.UpdateRelationships(context.Background(), []string{
		"organization:mhy#sre@user:bob",
	})
	assert.NoError(t, err)
	allowed, err = engine.Check(context.Background(), "user:bob", "edit", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = engine.Check(context.Background(), "user:bob", "read", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.True(t, allowed)

	// bob does not have permission (deleted all relationships)
	err = engine.DeleteAllSubjectRelationships(context.Background(), "user:bob")
	assert.NoError(t, err)
	allowed, err = engine.Check(context.Background(), "user:bob", "edit", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.False(t, allowed)
	allowed, err = engine.Check(context.Background(), "user:bob", "read", "DefaultResource:hc")
	assert.NoError(t, err)
	assert.False(t, allowed)
}
