// +build apitests

/*
	Copyright 2019 Netfoundry, Inc.

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

package tests

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/Jeffail/gabs"
	"github.com/google/uuid"
	"github.com/michaelquigley/pfxlog"
	"github.com/netfoundry/ziti-foundation/common/constants"
	"github.com/netfoundry/ziti-foundation/util/stringz"
	"github.com/pkg/errors"
	"gopkg.in/resty.v1"
	"net/http"
	"net/url"
	"sort"
)

type authenticator interface {
	Authenticate(ctx *TestContext) (*session, error)
}

type certAuthenticator struct {
	cert *x509.Certificate
	key  crypto.PrivateKey
}

func (authenticator *certAuthenticator) Authenticate(ctx *TestContext) (*session, error) {
	sess := &session{
		authenticator: authenticator,
		testContext:   ctx,
	}
	transport := ctx.Transport()
	transport.TLSClientConfig.Certificates = []tls.Certificate{
		{
			Certificate: [][]byte{authenticator.cert.Raw},
			PrivateKey:  authenticator.key,
		},
	}
	client := ctx.Client(ctx.HttpClient(transport))

	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		Post("/authenticate?method=cert")

	if err != nil {
		return nil, errors.Errorf("failed to authenticate via CERT: %s", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.Errorf("failed to authenticate via CERT: invalid response code encountered, got %d, expected %d", resp.StatusCode(), http.StatusOK)
	}

	if err = sess.parseSessionInfoFromResponse(resp); err != nil {
		return nil, err
	}

	return sess, nil
}

type updbAuthenticator struct {
	Username string
	Password string
}

func (authenticator *updbAuthenticator) Authenticate(ctx *TestContext) (*session, error) {
	sess := &session{
		authenticator: authenticator,
		testContext:   ctx,
	}

	sess.authenticatedRequests = authenticatedRequests{
		testContext: ctx,
		session:     sess,
	}

	body := gabs.New()
	_, _ = body.SetP(authenticator.Username, "username")
	_, _ = body.SetP(authenticator.Password, "password")

	resp, err := ctx.DefaultClient().R().
		SetHeader("Content-Type", "application/json").
		SetBody(body.String()).
		Post("/authenticate?method=password")

	if err != nil {
		return nil, errors.Errorf("failed to authenticate via UPDB as %v: %s", authenticator, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errors.Errorf("failed to authenticate via UPDB as %v: invalid response code encountered, got %d, expected %d", authenticator, resp.StatusCode(), http.StatusOK)
	}

	if err = sess.parseSessionInfoFromResponse(resp); err != nil {
		return nil, err
	}

	return sess, nil
}

type session struct {
	authenticator authenticator
	token         string
	identityId    string
	testContext   *TestContext
	authenticatedRequests
}

func (sess *session) newRequest(ctx *TestContext) *resty.Request {
	req := ctx.newRequest()
	if sess.token != "" {
		req.SetHeader(constants.ZitiSession, sess.token)
	}
	return req
}

func (sess *session) parseSessionInfoFromResponse(response *resty.Response) error {
	respBody := response.Body()

	if len(respBody) == 0 {
		return errors.Errorf("failed to authenticate via UPDB %v: encountered zero length body", sess.authenticator)
	}

	bodyContainer, err := gabs.ParseJSON(respBody)

	if err != nil {
		return errors.Errorf("failed to authenticate via UPDB as %v: failed to parse response: %s", sess.authenticator, err)
	}

	if sessionToken, ok := bodyContainer.Path("data.token").Data().(string); ok {
		sess.token = sessionToken
	} else {
		return errors.Errorf("failed to authenticate via UPDB as %v: failed to find session token", sess.authenticator)
	}

	if identityId, ok := bodyContainer.Path("data.identity.id").Data().(string); ok {
		sess.identityId = identityId
	} else {
		return errors.Errorf("failed to authenticate via UPDB as %v: failed to find identity id", sess.authenticator)
	}

	return nil
}

func (sess *session) logout() error {
	resp, err := sess.newRequest(sess.testContext).Delete("/current-api-session")

	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return errors.Errorf("could not delete current session %s for logout: got status code %d expected %d", sess.token, resp.StatusCode(), http.StatusOK)
	}

	return nil
}

type authenticatedRequests struct {
	testContext *TestContext
	session     *session
}

func (request *authenticatedRequests) newAuthenticatedRequest() *resty.Request {
	return request.session.newRequest(request.testContext)
}

func (request *authenticatedRequests) requireCreateIdentity(name string, password string, isAdmin bool, rolesAttributes ...string) string {
	entityData := gabs.New()
	request.testContext.setJsonValue(entityData, name, "name")
	request.testContext.setJsonValue(entityData, "User", "type")
	request.testContext.setJsonValue(entityData, isAdmin, "isAdmin")
	request.testContext.setJsonValue(entityData, rolesAttributes, "roleAttributes")

	enrollments := map[string]interface{}{
		"updb": name,
	}
	request.testContext.setJsonValue(entityData, enrollments, "enrollment")

	entityJson := entityData.String()
	httpCode, body := request.createEntityOfType("identities", entityJson)
	request.testContext.req.Equal(http.StatusCreated, httpCode)
	id := request.testContext.getEntityId(body)
	request.testContext.completeEnrollment(id, password)
	return id
}

func (request *authenticatedRequests) requireNewService(roleAttributes ...string) *testService {
	service := request.testContext.newTestService()
	service.roleAttributes = roleAttributes
	request.requireCreateEntity(service)
	return service
}

func (request *authenticatedRequests) requireNewEdgeRouter(roleAttributes ...string) *testEdgeRouter {
	edgeRouter := newTestEdgeRouter(roleAttributes...)
	request.requireCreateEntity(edgeRouter)
	return edgeRouter
}

func (request *authenticatedRequests) requireNewServicePolicy(policyType string, serviceRoles, identityRoles []string) *testServicePolicy {
	servicePolicy := newTestServicePolicy(policyType, serviceRoles, identityRoles)
	request.requireCreateEntity(servicePolicy)
	return servicePolicy
}

func (request *authenticatedRequests) requireNewEdgeRouterPolicy(edgeRouterRoles, identityRoles []string) *testEdgeRouterPolicy {
	edgeRouterPolicy := newTestEdgeRouterPolicy(edgeRouterRoles, identityRoles)
	request.requireCreateEntity(edgeRouterPolicy)
	return edgeRouterPolicy
}

func (request *authenticatedRequests) requireNewIdentity(isAdmin bool, roleAttributes ...string) *testIdentity {
	identity := newTestIdentity(isAdmin, roleAttributes...)
	request.requireCreateEntity(identity)
	return identity
}

func (request *authenticatedRequests) requireCreateEntity(entity testEntity) string {
	httpStatus, body := request.createEntity(entity)
	request.testContext.req.Equal(http.StatusCreated, httpStatus)
	id := request.testContext.getEntityId(body)
	entity.setId(id)
	return id
}

func (request *authenticatedRequests) requireDeleteEntity(entity testEntity) {
	httpStatus, _ := request.deleteEntityOfType(entity.getEntityType(), entity.getId())
	request.testContext.req.Equal(http.StatusOK, httpStatus)
}

func (request *authenticatedRequests) requireUpdateEntity(entity testEntity) {
	httpStatus, _ := request.updateEntity(entity)
	request.testContext.req.Equal(http.StatusOK, httpStatus)
}

func (request *authenticatedRequests) requireQuery(url string) *gabs.Container {
	httpStatus, body := request.query(url)
	request.testContext.logJson(body)
	request.testContext.req.Equal(http.StatusOK, httpStatus)
	return request.testContext.parseJson(body)
}

func (request *authenticatedRequests) requireAddAssociation(url string, ids ...string) {
	httpStatus, _ := request.addAssociation(url, ids...)
	request.testContext.req.Equal(http.StatusOK, httpStatus)
}

func (request *authenticatedRequests) requireRemoveAssociation(url string, ids ...string) {
	httpStatus, _ := request.removeAssociation(url, ids...)
	request.testContext.req.Equal(http.StatusOK, httpStatus)
}

func (request *authenticatedRequests) requireCreateNewService(roleAttributes ...string) *testService {
	service := request.testContext.newTestService(roleAttributes...)
	service.id = request.requireCreateEntity(service)
	return service
}

func (request *authenticatedRequests) createEntityOfType(entityType string, body string) (int, []byte) {
	resp, err := request.newAuthenticatedRequest().
		SetBody(body).
		Post("/" + entityType)

	request.testContext.req.NoError(err)
	request.testContext.logJson(resp.Body())
	return resp.StatusCode(), resp.Body()
}

func (request *authenticatedRequests) createEntity(entity testEntity) (int, []byte) {
	return request.createEntityOfType(entity.getEntityType(), entity.toJson(true, request.testContext))
}

func (request *authenticatedRequests) deleteEntityOfType(entityType string, id string) (int, []byte) {
	resp, err := request.newAuthenticatedRequest().Delete("/" + entityType + "/" + id)

	request.testContext.req.NoError(err)
	request.testContext.logJson(resp.Body())

	return resp.StatusCode(), resp.Body()
}

func (request *authenticatedRequests) updateEntity(entity testEntity) (int, []byte) {
	return request.updateEntityOfType(entity.getId(), entity.getEntityType(), entity.toJson(false, request.testContext), false)
}

func (request *authenticatedRequests) updateEntityOfType(id string, entityType string, body string, patch bool) (int, []byte) {
	if request.testContext.enabledJsonLogging {
		fmt.Printf("update body:\n%v\n", body)
	}

	urlPath := fmt.Sprintf("/%v/%v", entityType, id)
	pfxlog.Logger().Infof("url path: %v", urlPath)

	updateRequest := request.newAuthenticatedRequest().SetBody(body)

	var err error
	var resp *resty.Response

	if patch {
		resp, err = updateRequest.Patch(urlPath)
	} else {
		resp, err = updateRequest.Put(urlPath)
	}

	request.testContext.req.NoError(err)
	request.testContext.logJson(resp.Body())
	return resp.StatusCode(), resp.Body()
}

func (request *authenticatedRequests) query(url string) (int, []byte) {
	resp, err := request.newAuthenticatedRequest().Get("/" + url)
	request.testContext.req.NoError(err)
	return resp.StatusCode(), resp.Body()
}

func (request *authenticatedRequests) addAssociation(url string, ids ...string) (int, []byte) {
	return request.updateAssociation(http.MethodPut, url, ids...)
}

func (request *authenticatedRequests) removeAssociation(url string, ids ...string) (int, []byte) {
	return request.updateAssociation(http.MethodDelete, url, ids...)
}

func (request *authenticatedRequests) validateAssociations(entity testEntity, childType string, children ...testEntity) {
	var ids []string
	for _, child := range children {
		ids = append(ids, child.getId())
	}
	request.validateAssociationsAt(fmt.Sprintf("%v/%v/%v", entity.getEntityType(), entity.getId(), childType), ids...)
}

func (request *authenticatedRequests) validateAssociationContains(entity testEntity, childType string, children ...testEntity) {
	var ids []string
	for _, child := range children {
		ids = append(ids, child.getId())
	}
	request.validateAssociationsAtContains(fmt.Sprintf("%v/%v/%v", entity.getEntityType(), entity.getId(), childType), ids...)
}

func (request *authenticatedRequests) validateAssociationsAt(url string, ids ...string) {
	result := request.requireQuery(url)
	data := request.testContext.requirePath(result, "data")
	children, err := data.Children()

	var actualIds []string
	request.testContext.req.NoError(err)
	for _, child := range children {
		actualIds = append(actualIds, child.S("id").Data().(string))
	}

	sort.Strings(ids)
	sort.Strings(actualIds)
	request.testContext.req.Equal(ids, actualIds)
}

func (request *authenticatedRequests) validateAssociationsAtContains(url string, ids ...string) {
	result := request.requireQuery(url)
	data := request.testContext.requirePath(result, "data")
	children, err := data.Children()

	var actualIds []string
	request.testContext.req.NoError(err)
	for _, child := range children {
		actualIds = append(actualIds, child.S("id").Data().(string))
	}

	for _, id := range ids {
		request.testContext.req.True(stringz.Contains(actualIds, id), "%+v should contain %v", actualIds, id)
	}
}

func (request *authenticatedRequests) updateAssociation(method, url string, ids ...string) (int, []byte) {

	resp, err := request.newAuthenticatedRequest().
		SetBody(request.testContext.idsJson(ids...).String()).
		Execute(method, "/"+url)
	request.testContext.req.NoError(err)
	request.testContext.logJson(resp.Body())
	return resp.StatusCode(), resp.Body()
}

func (request *authenticatedRequests) isServiceVisibleToUser(serviceId string) bool {
	query := url.QueryEscape(fmt.Sprintf(`id = "%v"`, serviceId))
	result := request.requireQuery("services?filter=" + query)
	data := request.testContext.requirePath(result, "data")
	return nil != request.testContext.childWith(data, "id", serviceId)
}

func (request *authenticatedRequests) createUserAndLogin(isAdmin bool, roleAttributes ...string) *session {
	userAuth := &updbAuthenticator{
		Username: uuid.New().String(),
		Password: uuid.New().String(),
	}
	_ = request.requireCreateIdentity(userAuth.Username, userAuth.Password, isAdmin, roleAttributes...)
	session, _ := userAuth.Authenticate(request.testContext)

	return session
}

func (request *authenticatedRequests) validateEntityWithQuery(entity testEntity) *gabs.Container {
	query := url.QueryEscape(fmt.Sprintf(`id = "%v"`, entity.getId()))
	result := request.requireQuery(entity.getEntityType() + "?filter=" + query)
	data := request.testContext.requirePath(result, "data")
	jsonEntity := request.testContext.requireChildWith(data, "id", entity.getId())
	return request.testContext.validateEntity(entity, jsonEntity)
}

func (request *authenticatedRequests) validateEntityWithLookup(entity testEntity) *gabs.Container {
	result := request.requireQuery(entity.getEntityType() + "/" + entity.getId())
	jsonEntity := request.testContext.requirePath(result, "data")
	return request.testContext.validateEntity(entity, jsonEntity)
}

func (request *authenticatedRequests) validateUpdate(entity testEntity) *gabs.Container {
	result := request.requireQuery(entity.getEntityType() + "/" + entity.getId())
	jsonConfig := request.testContext.requirePath(result, "data")
	entity.validate(request.testContext, jsonConfig)
	return jsonConfig
}

func (request *authenticatedRequests) requireCreateNewConfig(data map[string]interface{}) *testConfig {
	config := request.testContext.newTestConfig(data)
	config.id = request.requireCreateEntity(config)
	return config
}

func (request *authenticatedRequests) requirePatchEntity(entity testEntity, fields ...string) {
	httpStatus, _ := request.patchEntity(entity, fields...)
	request.testContext.req.Equal(http.StatusOK, httpStatus)
}

func (request *authenticatedRequests) patchEntity(entity testEntity, fields ...string) (int, []byte) {
	return request.updateEntityOfType(entity.getId(), entity.getEntityType(), entity.toJson(false, request.testContext, fields...), true)
}
