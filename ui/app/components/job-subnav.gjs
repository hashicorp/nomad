/**
 * Copyright IBM Corp. 2015, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { service } from '@ember/service';
import Component from '@glimmer/component';
import { LinkTo } from '@ember/routing';
import didInsert from '@ember/render-modifiers/modifiers/did-insert';
import willDestroy from '@ember/render-modifiers/modifiers/will-destroy';
import formatJobId from 'nomad-ui/helpers/format-job-id';

export default class JobSubnav extends Component {
  @service abilities;
  @service keyboard;

  get shouldRenderClientsTab() {
    const { job } = this.args;
    return (
      job?.hasClientStatus &&
      !job?.hasChildren &&
      this.abilities.can('read client')
    );
  }

  get shouldHideNonParentTabs() {
    return this.args.job?.hasChildren;
  }

  get canListVariables() {
    return this.abilities.can('list variables');
  }

  <template>
    <div
      data-test-subnav="job"
      class="tabs is-subnav"
      {{didInsert this.keyboard.registerNav type="subnav"}}
      {{willDestroy this.keyboard.unregisterSubnav}}
    >
      <ul>
        <li data-test-tab="overview">
          <LinkTo
            @route="jobs.job.index"
            @model={{@job}}
            @activeClass="is-active"
            @current-when="jobs.job.index jobs.job.dispatch"
          >
            Overview
          </LinkTo>
        </li>
        <li data-test-tab="definition">
          <LinkTo
            @route="jobs.job.definition"
            @model={{@job}}
            @activeClass="is-active"
          >
            Definition
          </LinkTo>
        </li>
        <li data-test-tab="versions">
          <LinkTo
            @route="jobs.job.versions"
            @model={{@job}}
            @activeClass="is-active"
          >
            Versions
          </LinkTo>
        </li>
        {{#if @job.supportsDeployments}}
          <li data-test-tab="deployments">
            <LinkTo
              @route="jobs.job.deployments"
              @model={{@job}}
              @activeClass="is-active"
            >
              Deployments
            </LinkTo>
          </li>
        {{/if}}
        {{#unless this.shouldHideNonParentTabs}}
          <li data-test-tab="allocations">
            <LinkTo
              @route="jobs.job.allocations"
              @model={{formatJobId @job.id}}
              @activeClass="is-active"
            >
              Allocations
            </LinkTo>
          </li>
          <li data-test-tab="evaluations">
            <LinkTo
              @route="jobs.job.evaluations"
              @model={{@job}}
              @activeClass="is-active"
            >
              Evaluations
            </LinkTo>
          </li>
          {{#if this.shouldRenderClientsTab}}
            <li data-test-tab="clients">
              <LinkTo
                @route="jobs.job.clients"
                @model={{formatJobId @job.id}}
                @activeClass="is-active"
              >
                Clients
              </LinkTo>
            </li>
          {{/if}}
          <li data-test-tab="services">
            <LinkTo
              @route="jobs.job.services"
              @model={{@job}}
              @activeClass="is-active"
            >
              Services
            </LinkTo>
          </li>
        {{/unless}}
        {{#if this.canListVariables}}
          <li data-test-tab="variables">
            <LinkTo
              @route="jobs.job.variables"
              @model={{@job}}
              @activeClass="is-active"
            >
              Variables
            </LinkTo>
          </li>
        {{/if}}

      </ul>
    </div>
  </template>
}
