/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { LinkComponent } from '@ember/legacy-built-in-components';
import classic from 'ember-classic-decorator';

// Necessary for programmatic routing away pages with <LinkTo>s that contain @query properties.
// (There's an issue with query param calculations in the new component that uses the router service)
// https://github.com/emberjs/ember.js/issues/20051

@classic
export default class SafeLinkToComponent extends LinkComponent {}
