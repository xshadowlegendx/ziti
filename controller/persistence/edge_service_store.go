/*
	Copyright NetFoundry, Inc.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

	https://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
*/

package persistence

import (
	"github.com/michaelquigley/pfxlog"
	"github.com/openziti/edge/eid"
	"github.com/openziti/fabric/controller/db"
	"github.com/openziti/foundation/storage/ast"
	"github.com/openziti/foundation/storage/boltz"
	"github.com/openziti/foundation/util/errorz"
	"go.etcd.io/bbolt"
	"reflect"
)

const (
	FieldEdgeServiceDialIdentities = "dialIdentities"
	FieldEdgeServiceBindIdentities = "bindIdentities"
)

type EdgeService struct {
	db.Service
	RoleAttributes []string
	Configs        []string
}

func newEdgeService(name string, roleAttributes ...string) *EdgeService {
	return &EdgeService{
		Service: db.Service{
			BaseExtEntity: boltz.BaseExtEntity{Id: eid.New()},
			Name:          name,
		},
		RoleAttributes: roleAttributes,
	}
}

func (entity *EdgeService) LoadValues(store boltz.CrudStore, bucket *boltz.TypedBucket) {
	_, err := store.GetParentStore().BaseLoadOneById(bucket.Tx(), entity.Id, &entity.Service)
	bucket.SetError(err)

	entity.Name = bucket.GetStringOrError(FieldName)
	entity.RoleAttributes = bucket.GetStringList(FieldRoleAttributes)
	entity.Configs = bucket.GetStringList(EntityTypeConfigs)
}

func (entity *EdgeService) SetValues(ctx *boltz.PersistContext) {
	entity.Service.SetValues(ctx.GetParentContext())

	store := ctx.Store.(*edgeServiceStoreImpl)
	ctx.SetString(FieldName, entity.Name)
	ctx.SetStringList(FieldRoleAttributes, entity.RoleAttributes)
	ctx.SetLinkedIds(EntityTypeConfigs, entity.Configs)

	// index change won't fire if we don't have any roles on create, but we need to evaluate if we match any #all roles
	if ctx.IsCreate && len(entity.RoleAttributes) == 0 {
		store.rolesChanged(ctx.Bucket.Tx(), []byte(entity.Id), nil, nil, ctx.Bucket)
	}
}

func (entity *EdgeService) GetName() string {
	return entity.Name
}

type EdgeServiceStore interface {
	NameIndexedStore

	LoadOneById(tx *bbolt.Tx, id string) (*EdgeService, error)
	LoadOneByName(tx *bbolt.Tx, id string) (*EdgeService, error)
	IsBindableByIdentity(tx *bbolt.Tx, id string, identityId string) bool
	IsDialableByIdentity(tx *bbolt.Tx, id string, identityId string) bool
	GetRoleAttributesIndex() boltz.SetReadIndex
	GetRoleAttributesCursorProvider(values []string, semantic string) (ast.SetCursorProvider, error)
}

func newEdgeServiceStore(stores *stores) *edgeServiceStoreImpl {
	store := &edgeServiceStoreImpl{
		baseStore: newChildBaseStore(stores, stores.Service),
	}
	store.InitImpl(store)
	return store
}

type edgeServiceStoreImpl struct {
	*baseStore

	indexName           boltz.ReadIndex
	indexRoleAttributes boltz.SetReadIndex

	symbolRoleAttributes boltz.EntitySetSymbol
	symbolSessions       boltz.EntitySetSymbol
	symbolConfigs        boltz.EntitySetSymbol

	symbolServicePolicies           boltz.EntitySetSymbol
	symbolServiceEdgeRouterPolicies boltz.EntitySetSymbol

	symbolDialIdentities boltz.EntitySetSymbol
	symbolBindIdentities boltz.EntitySetSymbol
	symbolEdgeRouters    boltz.EntitySetSymbol

	bindIdentitiesCollection boltz.RefCountedLinkCollection
	dialIdentitiesCollection boltz.RefCountedLinkCollection
	edgeRoutersCollection    boltz.RefCountedLinkCollection
}

func (store *edgeServiceStoreImpl) NewStoreEntity() boltz.Entity {
	return &EdgeService{}
}

func (store *edgeServiceStoreImpl) GetRoleAttributesIndex() boltz.SetReadIndex {
	return store.indexRoleAttributes
}

func (store *edgeServiceStoreImpl) initializeLocal() {
	store.GetParentStore().GrantSymbols(store)

	store.symbolRoleAttributes = store.AddSetSymbol(FieldRoleAttributes, ast.NodeTypeString)

	store.indexName = store.GetParentStore().(db.ServiceStore).GetNameIndex()
	store.indexRoleAttributes = store.AddSetIndex(store.symbolRoleAttributes)

	store.symbolSessions = store.AddFkSetSymbol(EntityTypeSessions, store.stores.session)
	store.symbolServiceEdgeRouterPolicies = store.AddFkSetSymbol(EntityTypeServiceEdgeRouterPolicies, store.stores.serviceEdgeRouterPolicy)
	store.symbolServicePolicies = store.AddFkSetSymbol(EntityTypeServicePolicies, store.stores.servicePolicy)
	store.symbolConfigs = store.AddFkSetSymbol(EntityTypeConfigs, store.stores.config)

	store.symbolBindIdentities = store.AddFkSetSymbol(FieldEdgeServiceBindIdentities, store.stores.identity)
	store.symbolDialIdentities = store.AddFkSetSymbol(FieldEdgeServiceDialIdentities, store.stores.identity)
	store.symbolEdgeRouters = store.AddFkSetSymbol(migrationEntityTypeEdgeRouters, store.stores.edgeRouter)

	store.indexRoleAttributes.AddListener(store.rolesChanged)
}

func (store *edgeServiceStoreImpl) initializeLinked() {
	store.AddLinkCollection(store.symbolServiceEdgeRouterPolicies, store.stores.serviceEdgeRouterPolicy.symbolServices)
	store.AddLinkCollection(store.symbolServicePolicies, store.stores.servicePolicy.symbolServices)
	store.AddLinkCollection(store.symbolConfigs, store.stores.config.symbolServices)

	store.bindIdentitiesCollection = store.AddRefCountedLinkCollection(store.symbolBindIdentities, store.stores.identity.symbolBindServices)
	store.dialIdentitiesCollection = store.AddRefCountedLinkCollection(store.symbolDialIdentities, store.stores.identity.symbolDialServices)
	store.edgeRoutersCollection = store.AddRefCountedLinkCollection(store.symbolEdgeRouters, store.stores.edgeRouter.symbolServices)

	store.EventEmmiter.AddListener(boltz.EventUpdate, func(i ...interface{}) {
		if len(i) != 1 {
			return
		}
		service, ok := i[0].(*EdgeService)
		if !ok {
			pfxlog.Logger().Warnf("unexpected type in edge service event: %v", reflect.TypeOf(i[0]))
			return
		}
		store.stores.DbProvider.GetServiceCache().RemoveFromCache(service.Id)
		pfxlog.Logger().WithField("id", service).Debugf("removed service from fabric cache")
	})
}

func (store *edgeServiceStoreImpl) rolesChanged(tx *bbolt.Tx, rowId []byte, _ []boltz.FieldTypeAndValue, new []boltz.FieldTypeAndValue, holder errorz.ErrorHolder) {
	// Recalculate service policy links
	ctx := &roleAttributeChangeContext{
		tx:                    tx,
		rolesSymbol:           store.stores.servicePolicy.symbolServiceRoles,
		linkCollection:        store.stores.servicePolicy.serviceCollection,
		relatedLinkCollection: store.stores.servicePolicy.identityCollection,
		ErrorHolder:           holder,
	}
	store.updateServicePolicyRelatedRoles(ctx, rowId, new)

	// Recalculate service edge router policy links
	ctx = &roleAttributeChangeContext{
		tx:                    tx,
		rolesSymbol:           store.stores.serviceEdgeRouterPolicy.symbolServiceRoles,
		linkCollection:        store.stores.serviceEdgeRouterPolicy.serviceCollection,
		relatedLinkCollection: store.stores.serviceEdgeRouterPolicy.edgeRouterCollection,
		denormLinkCollection:  store.edgeRoutersCollection,
		ErrorHolder:           holder,
	}
	UpdateRelatedRoles(ctx, rowId, new, store.stores.serviceEdgeRouterPolicy.symbolSemantic)
}

func (store *edgeServiceStoreImpl) GetNameIndex() boltz.ReadIndex {
	return store.indexName
}

func (store *edgeServiceStoreImpl) LoadOneById(tx *bbolt.Tx, id string) (*EdgeService, error) {
	entity := &EdgeService{}
	if err := store.baseLoadOneById(tx, id, entity); err != nil {
		return nil, err
	}
	return entity, nil
}

func (store *edgeServiceStoreImpl) LoadOneByName(tx *bbolt.Tx, name string) (*EdgeService, error) {
	id := store.indexName.Read(tx, []byte(name))
	if id != nil {
		return store.LoadOneById(tx, string(id))
	}
	return nil, nil
}

func (store *edgeServiceStoreImpl) LoadOneByQuery(tx *bbolt.Tx, query string) (*EdgeService, error) {
	entity := &EdgeService{}
	if found, err := store.BaseLoadOneByQuery(tx, query, entity); !found || err != nil {
		return nil, err
	}
	return entity, nil
}

func (store *edgeServiceStoreImpl) DeleteById(ctx boltz.MutateContext, id string) error {
	for _, sessionId := range store.GetRelatedEntitiesIdList(ctx.Tx(), id, EntityTypeSessions) {
		if err := store.stores.session.DeleteById(ctx, sessionId); err != nil {
			return err
		}
	}

	if entity, _ := store.LoadOneById(ctx.Tx(), id); entity != nil {
		// Remove entity from ServiceRoles in service policies
		if err := store.deleteEntityReferences(ctx.Tx(), entity, store.stores.servicePolicy.symbolServiceRoles); err != nil {
			return err
		}

		// Remove entity from ServiceRoles in service edge router policies
		if err := store.deleteEntityReferences(ctx.Tx(), entity, store.stores.serviceEdgeRouterPolicy.symbolServiceRoles); err != nil {
			return err
		}
	}

	return store.BaseStore.DeleteById(ctx, id)
}

func (store *edgeServiceStoreImpl) GetRoleAttributesCursorProvider(values []string, semantic string) (ast.SetCursorProvider, error) {
	return store.getRoleAttributesCursorProvider(store.indexRoleAttributes, values, semantic)
}

func (store *edgeServiceStoreImpl) IsBindableByIdentity(tx *bbolt.Tx, id string, identityId string) bool {
	linkCount := store.bindIdentitiesCollection.GetLinkCount(tx, []byte(id), []byte(identityId))
	return linkCount != nil && *linkCount > 0
}

func (store *edgeServiceStoreImpl) IsDialableByIdentity(tx *bbolt.Tx, id string, identityId string) bool {
	linkCount := store.dialIdentitiesCollection.GetLinkCount(tx, []byte(id), []byte(identityId))
	return linkCount != nil && *linkCount > 0
}
