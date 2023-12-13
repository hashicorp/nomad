/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { task, timeout } from 'ember-concurrency';
import { classNames, classNameBindings } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@classNames('copy-button')
@classNameBindings('inset')
export default class CopyButton extends Component {
  clipboardText = null;
  state = null;

  @(task(function* () {
    this.set('state', 'success');

    yield timeout(2000);
    this.set('state', null);
  }).restartable())
  indicateSuccess;
}
