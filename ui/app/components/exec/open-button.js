/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@ember/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';
import { tagName } from '@ember-decorators/component';
import classic from 'ember-classic-decorator';

@classic
@tagName('')
export default class OpenButton extends Component {
  @service router;

  @action
  open() {
    openExecUrl(this.generateUrl());
  }

  generateUrl() {
    return generateExecUrl(this.router, {
      job: this.job,
      taskGroup: this.taskGroup,
      task: this.task,
      allocation: this.allocation,
    });
  }
}
