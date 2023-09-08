/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { classNames } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('job-deployment', 'boxed-section')
export default class JobDeployment extends Component {
  deployment = null;
  isOpen = false;
}
