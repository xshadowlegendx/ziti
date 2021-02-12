// Code generated by go-swagger; DO NOT EDIT.

//
// Copyright NetFoundry, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// __          __              _
// \ \        / /             (_)
//  \ \  /\  / /_ _ _ __ _ __  _ _ __   __ _
//   \ \/  \/ / _` | '__| '_ \| | '_ \ / _` |
//    \  /\  / (_| | |  | | | | | | | | (_| | : This file is generated, do not edit it.
//     \/  \/ \__,_|_|  |_| |_|_|_| |_|\__, |
//                                      __/ |
//                                     |___/

package rest_model

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// SessionDetail session detail
//
// swagger:model sessionDetail
type SessionDetail struct {
	BaseEntity

	// api session
	// Required: true
	APISession *EntityRef `json:"apiSession"`

	// api session Id
	// Required: true
	APISessionID *string `json:"apiSessionId"`

	// edge routers
	// Required: true
	EdgeRouters []*SessionEdgeRouter `json:"edgeRouters"`

	// route path
	// Required: true
	RoutePath []string `json:"routePath"`

	// service
	// Required: true
	Service *EntityRef `json:"service"`

	// service Id
	// Required: true
	ServiceID *string `json:"serviceId"`

	// token
	// Required: true
	Token *string `json:"token"`

	// type
	// Required: true
	Type DialBind `json:"type"`
}

// UnmarshalJSON unmarshals this object from a JSON structure
func (m *SessionDetail) UnmarshalJSON(raw []byte) error {
	// AO0
	var aO0 BaseEntity
	if err := swag.ReadJSON(raw, &aO0); err != nil {
		return err
	}
	m.BaseEntity = aO0

	// AO1
	var dataAO1 struct {
		APISession *EntityRef `json:"apiSession"`

		APISessionID *string `json:"apiSessionId"`

		EdgeRouters []*SessionEdgeRouter `json:"edgeRouters"`

		RoutePath []string `json:"routePath"`

		Service *EntityRef `json:"service"`

		ServiceID *string `json:"serviceId"`

		Token *string `json:"token"`

		Type DialBind `json:"type"`
	}
	if err := swag.ReadJSON(raw, &dataAO1); err != nil {
		return err
	}

	m.APISession = dataAO1.APISession

	m.APISessionID = dataAO1.APISessionID

	m.EdgeRouters = dataAO1.EdgeRouters

	m.RoutePath = dataAO1.RoutePath

	m.Service = dataAO1.Service

	m.ServiceID = dataAO1.ServiceID

	m.Token = dataAO1.Token

	m.Type = dataAO1.Type

	return nil
}

// MarshalJSON marshals this object to a JSON structure
func (m SessionDetail) MarshalJSON() ([]byte, error) {
	_parts := make([][]byte, 0, 2)

	aO0, err := swag.WriteJSON(m.BaseEntity)
	if err != nil {
		return nil, err
	}
	_parts = append(_parts, aO0)
	var dataAO1 struct {
		APISession *EntityRef `json:"apiSession"`

		APISessionID *string `json:"apiSessionId"`

		EdgeRouters []*SessionEdgeRouter `json:"edgeRouters"`

		RoutePath []string `json:"routePath"`

		Service *EntityRef `json:"service"`

		ServiceID *string `json:"serviceId"`

		Token *string `json:"token"`

		Type DialBind `json:"type"`
	}

	dataAO1.APISession = m.APISession

	dataAO1.APISessionID = m.APISessionID

	dataAO1.EdgeRouters = m.EdgeRouters

	dataAO1.RoutePath = m.RoutePath

	dataAO1.Service = m.Service

	dataAO1.ServiceID = m.ServiceID

	dataAO1.Token = m.Token

	dataAO1.Type = m.Type

	jsonDataAO1, errAO1 := swag.WriteJSON(dataAO1)
	if errAO1 != nil {
		return nil, errAO1
	}
	_parts = append(_parts, jsonDataAO1)
	return swag.ConcatJSON(_parts...), nil
}

// Validate validates this session detail
func (m *SessionDetail) Validate(formats strfmt.Registry) error {
	var res []error

	// validation for a type composition with BaseEntity
	if err := m.BaseEntity.Validate(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateAPISession(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateAPISessionID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateEdgeRouters(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateRoutePath(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateService(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateServiceID(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateToken(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateType(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *SessionDetail) validateAPISession(formats strfmt.Registry) error {

	if err := validate.Required("apiSession", "body", m.APISession); err != nil {
		return err
	}

	if m.APISession != nil {
		if err := m.APISession.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("apiSession")
			}
			return err
		}
	}

	return nil
}

func (m *SessionDetail) validateAPISessionID(formats strfmt.Registry) error {

	if err := validate.Required("apiSessionId", "body", m.APISessionID); err != nil {
		return err
	}

	return nil
}

func (m *SessionDetail) validateEdgeRouters(formats strfmt.Registry) error {

	if err := validate.Required("edgeRouters", "body", m.EdgeRouters); err != nil {
		return err
	}

	for i := 0; i < len(m.EdgeRouters); i++ {
		if swag.IsZero(m.EdgeRouters[i]) { // not required
			continue
		}

		if m.EdgeRouters[i] != nil {
			if err := m.EdgeRouters[i].Validate(formats); err != nil {
				if ve, ok := err.(*errors.Validation); ok {
					return ve.ValidateName("edgeRouters" + "." + strconv.Itoa(i))
				}
				return err
			}
		}

	}

	return nil
}

func (m *SessionDetail) validateRoutePath(formats strfmt.Registry) error {

	if err := validate.Required("routePath", "body", m.RoutePath); err != nil {
		return err
	}

	return nil
}

func (m *SessionDetail) validateService(formats strfmt.Registry) error {

	if err := validate.Required("service", "body", m.Service); err != nil {
		return err
	}

	if m.Service != nil {
		if err := m.Service.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("service")
			}
			return err
		}
	}

	return nil
}

func (m *SessionDetail) validateServiceID(formats strfmt.Registry) error {

	if err := validate.Required("serviceId", "body", m.ServiceID); err != nil {
		return err
	}

	return nil
}

func (m *SessionDetail) validateToken(formats strfmt.Registry) error {

	if err := validate.Required("token", "body", m.Token); err != nil {
		return err
	}

	return nil
}

func (m *SessionDetail) validateType(formats strfmt.Registry) error {

	if err := m.Type.Validate(formats); err != nil {
		if ve, ok := err.(*errors.Validation); ok {
			return ve.ValidateName("type")
		}
		return err
	}

	return nil
}

// MarshalBinary interface implementation
func (m *SessionDetail) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *SessionDetail) UnmarshalBinary(b []byte) error {
	var res SessionDetail
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
