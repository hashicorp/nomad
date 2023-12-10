/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from '@ember/routing/route';
import WithModelErrorHandling from 'nomad-ui/mixins/with-model-error-handling';

export default class ServerRoute extends Route.extend(WithModelErrorHandling) {}
