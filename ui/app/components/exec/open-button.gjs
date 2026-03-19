/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import { array } from '@ember/helper';
import { or } from 'ember-truth-helpers';
import { HdsButton } from '@hashicorp/design-system-components/components';
import cannot from 'ember-can/helpers/cannot';
import keyboardShortcutModifier from 'nomad-ui/modifiers/keyboard-shortcut';
import generateExecUrl from 'nomad-ui/utils/generate-exec-url';
import openExecUrl from 'nomad-ui/utils/open-exec-url';

export default class OpenButton extends Component {
  @service router;

  open = () => {
    openExecUrl(this.generateUrl());
  };

  generateUrl() {
    return generateExecUrl(this.router, {
      job: this.args.job,
      taskGroup: this.args.taskGroup,
      task: this.args.task,
      allocation: this.args.allocation,
    });
  }

  <template>
    {{#let
      (cannot "exec allocation" namespace=(or @job.namespaceId @job.namespace))
      as |cannotExec|
    }}
      <div
        class="exec-open-button"
        {{keyboardShortcutModifier
          label="Exec"
          pattern=(array "e" "x" "e" "c")
          action=this.open
        }}
      >
        <HdsButton
          data-test-exec-button
          @size="medium"
          @text="Exec"
          @color="secondary"
          disabled={{cannotExec}}
          {{on "click" this.open}}
          @icon="terminal-screen"
        />
      </div>
    {{/let}}
  </template>
}
