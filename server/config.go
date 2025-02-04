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

package server

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"stash.kopano.io/kc/konnect/config"
)

// Config defines a Server's configuration settings.
type Config struct {
	Config *config.Config

	Handler http.Handler
	Routes  []WithRoutes
}

// WithRoutes provide http routing withing a context.
type WithRoutes interface {
	AddRoutes(ctx context.Context, router *mux.Router)
}
