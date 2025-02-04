/*
 * Copyright 2017-2019 Kopano and its licensors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package authorities

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Registry implements the registry for registered authorities.
type Registry struct {
	mutex sync.RWMutex

	defaultID   string
	authorities map[string]*AuthorityRegistration

	logger logrus.FieldLogger
}

// NewRegistry creates a new authorizations Registry with the provided parameters.
func NewRegistry(ctx context.Context, registrationConfFilepath string, logger logrus.FieldLogger) (*Registry, error) {
	registryData := &RegistryData{}

	if registrationConfFilepath != "" {
		logger.Debugf("parsing authorities registration conf from %v", registrationConfFilepath)
		registryFile, err := ioutil.ReadFile(registrationConfFilepath)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(registryFile, registryData)
		if err != nil {
			return nil, err
		}
	}

	r := &Registry{
		authorities: make(map[string]*AuthorityRegistration),

		logger: logger,
	}

	var defaultAuthority *AuthorityRegistration
	for _, authority := range registryData.Authorities {
		validateErr := authority.Validate()
		registerErr := r.Register(authority)
		fields := logrus.Fields{
			"id":                 authority.ID,
			"client_id":          authority.ClientID,
			"with_client_secret": authority.ClientSecret != "",
			"authority_type":     authority.AuthorityType,
			"insecure":           authority.Insecure,
			"default":            authority.Default,
			"discover":           authority.discover,
			"alias_required":     authority.IdentityAliasRequired,
		}

		if validateErr != nil {
			logger.WithError(validateErr).WithFields(fields).Warnln("skipped registration of invalid authority entry")
			continue
		}
		if registerErr != nil {
			logger.WithError(registerErr).WithFields(fields).Warnln("skipped registration of invalid authority")
			continue
		}
		if authority.Default || defaultAuthority == nil {
			if defaultAuthority == nil || !defaultAuthority.Default {
				defaultAuthority = authority
			} else {
				logger.Warnln("ignored default authority flag since already have a default")
			}
		} else {
			// TODO(longsleep): Implement authority selection.
			logger.Warnln("non-default additional authorities are not supported yet")
		}

		go authority.Initialize(ctx, logger)

		logger.WithFields(fields).Debugln("registered authority")
	}

	if defaultAuthority != nil {
		if defaultAuthority.Default {
			r.defaultID = defaultAuthority.ID
			logger.WithField("id", defaultAuthority.ID).Infoln("using external default authority")
		} else {
			logger.Warnln("non-default authorities are not supported yet")
		}
	}

	return r, nil
}

// Register validates the provided authority registration and adds the authority
// to the accociated registry if valid. Returns error otherwise.
func (r *Registry) Register(authority *AuthorityRegistration) error {
	if authority.ID == "" {
		if authority.Name != "" {
			authority.ID = authority.Name
			r.logger.WithField("id", authority.ID).Warnln("authority has no id, using name")
		} else {
			return errors.New("no authority id")
		}
	}
	if authority.ClientID == "" {
		return errors.New("invalid authority client_id")
	}

	switch authority.AuthorityType {
	case AuthorityTypeOIDC:
		// Ensure some defaults.
		if len(authority.Scopes) == 0 {
			authority.Scopes = authorityDefaultScopes
		}
		if authority.ResponseType == "" {
			authority.ResponseType = authorityDefaultResponseType
		}
		if authority.CodeChallengeMethod == "" {
			authority.CodeChallengeMethod = authorityDefaultCodeChallengeMethod
		}
		if authority.IdentityClaimName == "" {
			authority.IdentityClaimName = authorityDefaultIdentityClaimName
		}

	default:
		return fmt.Errorf("unknown authority type: %v", authority.AuthorityType)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.authorities[authority.ID] = authority

	return nil
}

// Lookup returns and validates the authority Detail information for the provided
// parameters from the accociated authority registry.
func (r *Registry) Lookup(ctx context.Context, authorityID string) (*Details, error) {
	registration, ok := r.Get(ctx, authorityID)
	if !ok {
		return nil, fmt.Errorf("unknown authority id: %v", authorityID)
	}

	// Create immutable registry record.
	// TODO(longsleep): Cache record.
	details := &Details{
		ID:            registration.ID,
		Name:          registration.Name,
		AuthorityType: registration.AuthorityType,

		ClientID:     registration.ClientID,
		ClientSecret: registration.ClientSecret,

		Insecure: registration.Insecure,

		Scopes:              registration.Scopes,
		ResponseType:        registration.ResponseType,
		CodeChallengeMethod: registration.CodeChallengeMethod,

		Registration: registration,
	}
	registration.mutex.RLock()
	// Fill in dynamic stuff.
	details.ready = registration.ready
	if registration.ready {
		details.AuthorizationEndpoint = registration.authorizationEndpoint
		details.validationKeys = registration.validationKeys
	}
	registration.mutex.RUnlock()

	return details, nil
}

// Get returns the registered authorities registration for the provided client ID.
func (r *Registry) Get(ctx context.Context, authorityID string) (*AuthorityRegistration, bool) {
	if authorityID == "" {
		return nil, false
	}

	// Lookup authority registration.
	r.mutex.RLock()
	registration, ok := r.authorities[authorityID]
	r.mutex.RUnlock()

	return registration, ok
}

// Default returns the default authority from the associated registry if any.
func (r *Registry) Default(ctx context.Context) *Details {
	authority, _ := r.Lookup(ctx, r.defaultID)
	return authority
}
