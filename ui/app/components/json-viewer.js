/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@ember/component';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('json-viewer')
@classNameBindings('fluidHeight:has-fluid-height')
export default class JsonViewer extends Component {}
