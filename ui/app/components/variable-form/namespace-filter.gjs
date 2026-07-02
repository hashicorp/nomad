/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { fn } from '@ember/helper';
import { on } from '@ember/modifier';
import { didInsert } from '@ember/render-modifiers';
import { eq } from 'ember-truth-helpers';
import { HdsDropdown } from '@hashicorp/design-system-components/components';
import Trigger from 'nomad-ui/components/trigger';

export default class NamespaceFilter extends Component {
  @service store;

  fetchNamespaces = async () => {
    return this.store.findAll('namespace');
  };

  formatAndSetNamespaces = () => {
    const namespaces = this.store
      .peekAll('namespace')
      .map(({ name }) => ({ key: name, label: name }));

    if (namespaces.length <= 1) return null;

    this.args.fns.setNamespaceOptions(namespaces);
  };

  <template>
    <Trigger
      @do={{this.fetchNamespaces}}
      @onSuccess={{this.formatAndSetNamespaces}}
      as |trigger|
    >
      <span hidden {{didInsert trigger.fns.do}}></span>
      {{#if trigger.data.isSuccess}}
        {{#if trigger.data.result}}
          {{#if @data.namespaceOptions}}
            <HdsDropdown
              class="namespace-dropdown"
              data-test-variable-namespace-filter
              as |dd|
            >
              <dd.ToggleButton
                @text={{@data.selection}}
                @color="secondary"
                disabled={{@data.disabled}}
                @isFullWidth={{true}}
              />
              {{#each @data.namespaceOptions as |option|}}
                <dd.Radio
                  name={{option.key}}
                  {{on "change" (fn @fns.onSelect option.key)}}
                  checked={{eq @data.selection option.key}}
                >
                  {{option.label}}
                </dd.Radio>
              {{/each}}
            </HdsDropdown>
          {{/if}}
        {{/if}}
      {{/if}}
    </Trigger>
  </template>
}
