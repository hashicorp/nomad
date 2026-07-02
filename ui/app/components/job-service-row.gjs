/**
 * Copyright IBM Corp. 2015, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { service } from '@ember/service';
import { on } from '@ember/modifier';
import { fn, hash } from '@ember/helper';
import { LinkTo } from '@ember/routing';
import { and, eq } from 'ember-truth-helpers';
import { HdsIcon } from '@hashicorp/design-system-components/components';
import keyboardShortcut from 'nomad-ui/modifiers/keyboard-shortcut';

export default class JobServiceRow extends Component {
  @service router;
  @service system;

  gotoService = (service) => {
    if (service.provider === 'nomad') {
      this.router.transitionTo('jobs.job.services.service', service.name, {
        queryParams: { level: service.level },
        instances: service.instances,
      });
    }
  };

  get consulRedirectLink() {
    return this.system.agent.get('config')?.UI?.Consul?.BaseUIURL;
  }

  get instancesCount() {
    return this.args.service?.instances?.length ?? 0;
  }

  get allocationLabel() {
    return this.instancesCount === 1 ? 'allocation' : 'allocations';
  }

  <template>
    <tr
      data-test-service-row
      data-test-service-name={{@service.name}}
      data-test-num-allocs={{this.instancesCount}}
      data-test-service-provider={{@service.provider}}
      data-test-service-level={{@service.level}}
      {{on "click" (fn this.gotoService @service)}}
      class={{if (eq @service.provider "nomad") "is-interactive"}}
      ...attributes
    >
      <td
        {{keyboardShortcut
          enumerated=true
          action=(fn this.gotoService @service)
        }}
      >
        {{#if (eq @service.provider "nomad")}}
          <HdsIcon @name="nomad-color" @isInline={{true}} />
          <LinkTo
            class="is-primary"
            @route="jobs.job.services.service"
            @model={{@service}}
            @query={{hash level=@service.level}}
          >{{@service.name}}</LinkTo>
        {{else}}
          <HdsIcon @name="consul-color" @isInline={{true}} />
          {{#if (and (eq @service.provider "consul") this.consulRedirectLink)}}
            <a
              class="is-primary"
              href={{this.consulRedirectLink}}
              target="_blank"
              rel="noopener noreferrer"
            >
              {{@service.name}}
            </a>
          {{else}}
            {{@service.name}}
          {{/if}}
          {{#if @service.connect}}
            <HdsIcon @name="mesh" @color="#444444" @isInline={{true}} />
          {{/if}}
        {{/if}}
      </td>
      <td>
        {{@service.level}}
      </td>
      <td>
        {{#each @service.tags as |tag|}}
          <span class="tag is-service">{{tag}}</span>
        {{/each}}
        {{#each @service.canary_tags as |tag|}}
          <span class="tag canary is-service">{{tag}}</span>
        {{/each}}
      </td>
      <td>
        {{#if (eq @service.provider "nomad")}}
          {{this.instancesCount}}
          {{this.allocationLabel}}
        {{else}}
          --
        {{/if}}
      </td>
    </tr>
  </template>
}
